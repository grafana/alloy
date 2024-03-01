import React from 'react';

/**
 * PathPrefixContext propagates the base URL throughout the component tree where
 * the application is hosted.
 */
const PathPrefixContext = React.createContext('');

/**
 * usePathPrefix retrieves the current base URL where the application is
 * hosted. Links and API calls should be all relative to this path. Returns
 * `/` if there is no path prefix.
 *
 * The returned path prefix will always end in a `/`.
 */
function usePathPrefix(): string {
  const prefix = React.useContext(PathPrefixContext);
  if (prefix === '') {
    return '/';
  }

  if (prefix.endsWith('/')) {
    return prefix;
  }
  return prefix + '/';
}

export { PathPrefixContext, usePathPrefix };
