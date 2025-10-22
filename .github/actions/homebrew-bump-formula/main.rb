# frozen_string_literal: true

require 'formula'

class Object
  def false?
    nil?
  end
end

class String
  def false?
    empty? || strip == 'false'
  end
end

module Homebrew
  module_function

  def print_command(*cmd)
    puts "[command]#{cmd.join(' ').gsub("\n", ' ')}"
  end

  def brew(*args)
    print_command ENV["HOMEBREW_BREW_FILE"], *args
    safe_system ENV["HOMEBREW_BREW_FILE"], *args
  end

  def git(*args)
    print_command ENV["HOMEBREW_GIT"], *args
    safe_system ENV["HOMEBREW_GIT"], *args
  end

  def read_brew(*args)
    print_command ENV["HOMEBREW_BREW_FILE"], *args
    output = `#{ENV["HOMEBREW_BREW_FILE"]} #{args.join(' ')}`.chomp
    odie output if $CHILD_STATUS.exitstatus != 0
    output
  end

  # Get inputs
  message = ENV['HOMEBREW_BUMP_MESSAGE']
  user_name = ENV['HOMEBREW_GIT_NAME']
  user_email = ENV['HOMEBREW_GIT_EMAIL']
  org = ENV['HOMEBREW_BUMP_ORG']
  no_fork = ENV['HOMEBREW_BUMP_NO_FORK']
  tap = ENV['HOMEBREW_BUMP_TAP']
  tap_url = ENV['HOMEBREW_BUMP_TAP_URL']
  formula = ENV['HOMEBREW_BUMP_FORMULA']
  tag = ENV['HOMEBREW_BUMP_TAG']
  revision = ENV['HOMEBREW_BUMP_REVISION']
  force = ENV['HOMEBREW_BUMP_FORCE']
  livecheck = ENV['HOMEBREW_BUMP_LIVECHECK']

  # Check inputs
  if livecheck.false?
    odie "Need 'formula' input specified" if formula.blank?
    odie "Need 'tag' input specified" if tag.blank?
  end

  # Avoid using the GitHub API whenever possible.
  # This helps users who use application tokens instead of personal access tokens.
  # Application tokens don't work with GitHub API's `/user` endpoit.
  if user_name.blank? && user_email.blank?
    # Get user details
    user = GitHub::API.open_rest "#{GitHub::API_URL}/user"
    user_id = user['id']
    user_login = user['login']
    user_name = user['name'] || user['login'] if user_name.blank?
    user_email = user['email'] || (
      # https://help.github.com/en/github/setting-up-and-managing-your-github-user-account/setting-your-commit-email-address
      user_created_at = Date.parse user['created_at']
      plus_after_date = Date.parse '2017-07-18'
      need_plus_email = (user_created_at - plus_after_date).positive?
      user_email = "#{user_login}@users.noreply.github.com"
      user_email = "#{user_id}+#{user_email}" if need_plus_email
      user_email
    ) if user_email.blank?
  end

  # Tell git who you are
  git 'config', '--global', 'user.name', user_name
  git 'config', '--global', 'user.email', user_email

  if tap.blank?
    brew 'tap', 'homebrew/core', '--force'
  else
    # Tap the requested tap if applicable
    brew 'tap', tap, *(tap_url unless tap_url.blank?)
  end

  # Append additional PR message
  message = if message.blank?
              ''
            else
              message + "\n\n"
            end
  message += '[`action-homebrew-bump-formula`](https://github.com/dawidd6/action-homebrew-bump-formula)'

  unless force.false?
    brew_repo = read_brew '--repository'
    git '-C', brew_repo, 'apply', "#{__dir__}/bump-formula-pr.rb.patch"
  end

  # Do the livecheck stuff or not
  if livecheck.false?
    # Change formula name to full name
    formula = tap + '/' + formula if !tap.blank? && !formula.blank?

    # Get info about formula
    stable = Formula[formula].stable
    is_git = stable.downloader.is_a? GitDownloadStrategy

    # Prepare tag and url
    tag = tag.delete_prefix 'refs/tags/'
    version = Version.parse tag

    # Finally bump the formula
    brew 'bump-formula-pr',
         '--no-audit',
         '--no-browse',
         "--message=#{message}",
         *("--fork-org=#{org}" unless org.blank?),
         *("--no-fork" unless no_fork.false?),
         *("--version=#{version}" unless is_git),
         *("--tag=#{tag}" if is_git),
         *("--revision=#{revision}" if is_git),
         *("--force" unless force.false?),
         formula
  else
    # Support multiple formulae in input and change to full names if tap
    unless formula.blank?
      formula = formula.split(/[ ,\n]/).reject(&:blank?)
      formula = formula.map { |f| tap + '/' + f } unless tap.blank?
    end

    # Get livecheck info
    json = read_brew 'livecheck',
                     '--formula',
                     '--quiet',
                     '--newer-only',
                     '--full-name',
                     '--json',
                     *("--tap=#{tap}" if !tap.blank? && formula.blank?),
                     *(formula unless formula.blank?)
    json = JSON.parse json

    # Define error
    err = nil

    # Loop over livecheck info
    json.each do |info|
      # Skip if there is no version field
      next unless info['version']

      # Get info about formula
      formula = info['formula']
      version = info['version']['latest']

      begin
        # Finally bump the formula
        brew 'bump-formula-pr',
             '--no-audit',
             '--no-browse',
             "--message=#{message}",
             "--version=#{version}",
             *("--fork-org=#{org}" unless org.blank?),
             *("--no-fork" unless no_fork.false?),
             *("--force" unless force.false?),
             formula
      rescue ErrorDuringExecution => e
        # Continue execution on error, but save the exeception
        err = e
      end
    end

    # Die if error occured
    odie err if err
  end
end
