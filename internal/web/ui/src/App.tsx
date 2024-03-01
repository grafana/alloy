import { PathPrefixContext } from './contexts/PathPrefixContext';
import Router from './Router';

import styles from './App.module.css';

/**
 * getBasePath retrieves the base path of the application by looking at the
 * <base> HTML element in the HTML header. If there is no <base> element or the
 * <base> element is empty, getBaseURL returns "/".
 */
function getBasePath(): string {
  const elements = document.getElementsByTagName('base');
  if (elements.length !== 1) {
    return '/';
  }

  // elements[0].href will be a full URL, but we just want to extract the path
  // portion.
  return new URL(elements[0].href).pathname;
}

function App() {
  const basePath = getBasePath();

  return (
    <PathPrefixContext.Provider value={basePath}>
      <div className={styles.app}>
        <Router basePath={basePath} />
      </div>
    </PathPrefixContext.Provider>
  );
}

export default App;
