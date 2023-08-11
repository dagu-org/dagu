import React from 'react';
import { createRoot } from 'react-dom/client';

import App, { Config } from './App';
import './styles/styles.css';
import './styles/prism.css';

import "@fontsource/inter"
import "@fontsource/inter/300.css"
import "@fontsource/inter/400.css"
import "@fontsource/inter/500.css"
import "@fontsource/inter/600.css"
import '@fontsource/roboto/300.css';
import '@fontsource/roboto/400.css';
import '@fontsource/roboto/500.css';
import '@fontsource/roboto/700.css';

declare global {
  const getConfig: () => Config;
}

const container = document.getElementById('root');
const root = createRoot(container!);
const config = getConfig();
root.render(<App config={config} />);
