import * as React from "react";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import LayersIcon from "@mui/icons-material/Layers";
import FilterAltIcon from '@mui/icons-material/FilterAlt';
import DashboardIcon from "@mui/icons-material/Dashboard";
import { Link } from "react-router-dom";

export const mainListItems = (
  <React.Fragment>
    <Link to="/">
      <ListItemButton>
        <ListItemIcon>
          <DashboardIcon />
        </ListItemIcon>
        <ListItemText primary="Dashboard" />
      </ListItemButton>
    </Link>
    <Link to="/dags">
      <ListItemButton>
        <ListItemIcon>
          <LayersIcon />
        </ListItemIcon>
        <ListItemText primary="DAGs" />
      </ListItemButton>
    </Link>
    <Link to="/views">
      <ListItemButton>
        <ListItemIcon>
          <FilterAltIcon />
        </ListItemIcon>
        <ListItemText primary="Views" />
      </ListItemButton>
    </Link>
  </React.Fragment>
);
