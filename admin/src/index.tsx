import React from "react";
import { createRoot } from "react-dom/client";

import App from "./App";
import "bulma/css/bulma.css";
import "@fortawesome/fontawesome-free/css/all.min.css";
import '@fortawesome/fontawesome-free/js/all.min.js'
import "./styles/styles.css";
import "./styles/prism.css";

const container = document.getElementById("root");
const root = createRoot(container!);
//@ts-ignore
const config = getConfig();
root.render(<App config={config} />);
