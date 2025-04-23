# Tag Release

A tag is required to create GitHub artifacts and as a prerequisite for publishing.

## Before you begin

1. All required commits for the release should exist on the release branch. This includes functionality and documentation such as the `CHANGELOG.md`. All versions in code should have already been updated.

2. Make sure you are up to date on the release branch:

   ``` 
   git checkout release-VERSION_PREFIX
   git fetch origin 
   git pull origin 
   ```

3. Determine the [VERSION](concepts/version.md).

4. Follow the GitHub [instructions](https://docs.github.com/en/authentication/managing-commit-signature-verification) to set up GPG for signature verification.

5. Optional: Configure git to always sign on commit or tag.

```bash
git config --global commit.gpgSign true
git config --global tag.gpgSign true
```

If you are on macOS or linux and using an encrypted GPG key, `gpg-agent` or `gpg` may be unable
to prompt you for your private key passphrase. This will be denoted by an error
when creating a commit or tag. To circumvent the error, add the following into
your `~/.bash_profile`, `~/.bashrc` or `~/.zshrc`, depending on which shell you are using.

```
export GPG_TTY=$(tty)
```

## Steps

1. Tag the release:

    Example commands:

    ```
    git tag -s VERSION
    git push origin VERSION
    ```

2. After a tag has been pushed, GitHub Tasks will create release assets and open a release draft for every pushed tag.

    - This will take ~20-40 minutes.
    - You can monitor this by viewing the drone build on the commit for the release tag.

    If the Homebrew Formula fails to update, close the existing open PR and re-run the failed CI.