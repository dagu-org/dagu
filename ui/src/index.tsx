import { createRoot } from 'react-dom/client';

import App from './App';
import { CookiesProvider } from 'react-cookie';
import './styles/prism.css';
import './styles/global.css';

import { Config } from './contexts/ConfigContext';

declare global {
  const getConfig: () => Config;
}

const container = document.getElementById('root');
const root = createRoot(container!);
const config = getConfig();
root.render(
  <CookiesProvider>
    <App config={config} />
  </CookiesProvider>
);
