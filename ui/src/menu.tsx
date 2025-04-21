import * as React from 'react';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import { Link } from 'react-router-dom';
import { Typography } from '@mui/material';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faChartGantt,
  faMagnifyingGlass,
  faTableList,
} from '@fortawesome/free-solid-svg-icons';
import { IconProp } from '@fortawesome/fontawesome-svg-core';

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
        style={{ height: 20, width: 20 }}
        icon={icon}
      ></FontAwesomeIcon>
    </span>
  );
}

export const mainListItems = (
  <React.Fragment>
    <Link to="/dashboard">
      <ListItem text="Dashboard" icon={<Icon icon={faChartGantt}></Icon>} />
    </Link>
    <Link to="/dags">
      <ListItem text="DAGs" icon={<Icon icon={faTableList}></Icon>} />
    </Link>
    <Link to="/search">
      <ListItem text="Search" icon={<Icon icon={faMagnifyingGlass}></Icon>} />
    </Link>
  </React.Fragment>
);

type ListItemProps = {
  icon: React.ReactNode;
  text: string;
};

function ListItem({ icon, text }: ListItemProps) {
  return (
    <ListItemButton>
      <ListItemIcon sx={{ color: 'white' }}>{icon}</ListItemIcon>
      <ListItemText
        primary={
          <Typography
            sx={{
              color: 'white',
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
