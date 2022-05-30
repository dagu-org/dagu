import React from "react";
import { createRoot } from "react-dom/client";

import App from "./App";
import "bulma/css/bulma.min.css"
import "@fortawesome/fontawesome-free/css/all.min.css";
import "@fortawesome/fontawesome-free/js/all.min.js";
import "./styles/styles.css";
import "./styles/prism.css";

import '@fontsource/roboto/300.css';
import '@fontsource/roboto/400.css';
import '@fontsource/roboto/500.css';
import '@fontsource/roboto/700.css';

const container = document.getElementById("root");
const root = createRoot(container!);
//@ts-ignore
const config = getConfig();
root.render(<App config={config} />);
