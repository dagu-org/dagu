import * as React from 'react';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faChartGantt,
  faMagnifyingGlass,
  faTableList,
  faTerminal,
} from '@fortawesome/free-solid-svg-icons';
import { IconProp } from '@fortawesome/fontawesome-svg-core';
import { Typography } from '@mui/material';

function Icon({ icon }: { icon: IconProp }) {
  return (
    <span
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        marginLeft: 2,
      }}
    >
      <FontAwesomeIcon
        style={{ height: 20, width: 20, color:'#45B8AC' }}
        icon={icon}
      ></FontAwesomeIcon>
    </span>
  );
}

export const mainListItems = (
  <React.Fragment>
    <ListItem
      text="Dashboard"
      icon={<Icon icon={faChartGantt} />}
      to="/dashboard"
    />
    <ListItem
      text="DAGs"
      icon={<Icon icon={faTableList} />}
      to="/dags"
    />
    <ListItem
      text="Search"
      icon={<Icon icon={faMagnifyingGlass} />}
      to="/search"
    />
    <ListItem
      text="Terminal"
      icon={<Icon icon={faTerminal} />}
      to="http://localhost:8090"
      external
    />
  </React.Fragment>
);

type ListItemProps = {
  icon: React.ReactNode;
  text: string;
  to: string;
  external?: boolean;
};

function ListItem({ icon, text, to, external }: ListItemProps) {
  const listItemProps = external ? { href: to, target: '_blank' } : { to };

  return (
    <ListItemButton component="a" {...listItemProps}>
      <ListItemIcon sx={{ color: 'black' }}>{icon}</ListItemIcon>
      <ListItemText
        primary={
          <Typography
            sx={{
              color: 'black',
              fontWeight: '400',
            }}
          >
            {text}
          </Typography>
        }
      />
    </ListItemButton>
  );
}
