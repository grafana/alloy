import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import App from './App.tsx';
import { createTheme, ThemeContext } from '@grafana/data';

createRoot(document.getElementById('root')!).render(
  <ThemeContext.Provider value={createTheme({ colors: { mode: 'light' } })}>
    <StrictMode>
      <App />
    </StrictMode>
    ,
  </ThemeContext.Provider>
);
