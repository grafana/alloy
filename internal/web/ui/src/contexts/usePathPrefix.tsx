import { useContext } from 'react';

import { PathPrefixContext } from './PathPrefixContext';

/**
 * usePathPrefix retrieves the current base URL where the application is
 * hosted. Links and API calls should be all relative to this path. Returns
 * `/` if there is no path prefix.
 *
 * The returned path prefix will always end in a `/`.
 */
function usePathPrefix(): string {
  const prefix = useContext(PathPrefixContext);
  if (prefix === '') {
    return '/';
  }

  if (prefix.endsWith('/')) {
    return prefix;
  }
  return prefix + '/';
}

export { usePathPrefix };
