import * as React from 'react';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import { Link } from 'react-router-dom';
import { TimelineOutlined, TocOutlined } from '@mui/icons-material';

export const mainListItems = (
  <React.Fragment>
    <Link to="/">
      <ListItemButton>
        <ListItemIcon>
          <TimelineOutlined />
        </ListItemIcon>
        <ListItemText primary="Dashboard" />
      </ListItemButton>
    </Link>
    <Link to="/dags">
      <ListItemButton>
        <ListItemIcon>
          <TocOutlined />
        </ListItemIcon>
        <ListItemText primary="DAGs" />
      </ListItemButton>
    </Link>
  </React.Fragment>
);
