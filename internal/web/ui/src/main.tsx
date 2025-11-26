import 'normalize.css';
import './index.css';
import 'rc-slider/assets/index.css';

import { createTheme, ThemeContext } from '@grafana/data';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import App from './App';

createRoot(document.getElementById('root')!).render(
  <ThemeContext.Provider value={createTheme({ colors: { mode: 'light' } })}>
    <StrictMode>
      <App />
    </StrictMode>
  </ThemeContext.Provider>
);
