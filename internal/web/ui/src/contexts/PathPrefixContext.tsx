import { createContext } from 'react';

/**
 * PathPrefixContext propagates the base URL throughout the component tree where
 * the application is hosted.
 */
const PathPrefixContext = createContext('');

export { PathPrefixContext };
