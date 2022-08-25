import * as React from 'react';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import { Link } from 'react-router-dom';
import {
  TimelineOutlined,
  TocOutlined,
  SearchOutlined,
} from '@mui/icons-material';
import { Typography } from '@mui/material';

export const mainListItems = (
  <React.Fragment>
    <Link to="/">
      <ListItem text="Dashboard" icon={<TimelineOutlined />} />
    </Link>
    <Link to="/dags">
      <ListItem text="DAGs" icon={<TocOutlined />} />
    </Link>
    <Link to="/search">
      <ListItem text="Search" icon={<SearchOutlined />} />
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
      <ListItemIcon
        sx={{
          color: 'white',
        }}
      >
        {icon}
      </ListItemIcon>
      <ListItemText
        primary={
          <Typography
            sx={{
              color: 'white',
              fontWeight: '600',
            }}
          >
            {text}
          </Typography>
        }
      />
    </ListItemButton>
  );
}
