import React from 'react';
import ReactDOM from 'react-dom/client';

import { createTheme } from '@grafana/data';
import { ThemeContext } from '@grafana/ui';

import App from './App';

import 'normalize.css';
import './index.css';
import 'rc-slider/assets/index.css';

const root = ReactDOM.createRoot(document.getElementById('root') as HTMLElement);
root.render(
  <ThemeContext.Provider value={createTheme({ colors: { mode: 'light' } })}>
    <React.StrictMode>
      <App />
    </React.StrictMode>
  </ThemeContext.Provider>
);
