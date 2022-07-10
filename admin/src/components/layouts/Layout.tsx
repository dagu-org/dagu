import * as React from 'react';
import { styled, createTheme, ThemeProvider } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import MuiDrawer from '@mui/material/Drawer';
import Box from '@mui/material/Box';
import MuiAppBar, { AppBarProps as MuiAppBarProps } from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import List from '@mui/material/List';
import Typography from '@mui/material/Typography';
import { mainListItems } from '../../menu';
import { Grid, IconButton } from '@mui/material';
import icon from '../../assets/images/dagu.png';

const drawerWidthClosed = 64;
const drawerWidth = 240;

interface AppBarProps extends MuiAppBarProps {
  open?: boolean;
}

const AppBar = styled(MuiAppBar, {
  shouldForwardProp: (prop) => prop !== 'open',
})<AppBarProps>(({ theme, open }) => ({
  zIndex: theme.zIndex.drawer - 1,
  transition: theme.transitions.create(['width', 'margin'], {
    easing: theme.transitions.easing.sharp,
    duration: theme.transitions.duration.leavingScreen,
  }),
  marginLeft: drawerWidthClosed,
  width: `calc(100% - ${drawerWidthClosed}px)`,
  ...(open && {
    marginLeft: drawerWidth,
    width: `calc(100% - ${drawerWidth}px)`,
    transition: theme.transitions.create(['width', 'margin'], {
      easing: theme.transitions.easing.sharp,
      duration: theme.transitions.duration.enteringScreen,
    }),
  }),
}));

const Drawer = styled(MuiDrawer, {
  shouldForwardProp: (prop) => prop !== 'open',
})(({ theme, open }) => ({
  '& .MuiDrawer-paper': {
    position: 'relative',
    whiteSpace: 'nowrap',
    width: drawerWidth,
    transition: theme.transitions.create('width', {
      easing: theme.transitions.easing.sharp,
      duration: theme.transitions.duration.enteringScreen,
    }),
    boxSizing: 'border-box',
    ...(!open && {
      overflowX: 'hidden',
      transition: theme.transitions.create('width', {
        easing: theme.transitions.easing.sharp,
        duration: theme.transitions.duration.leavingScreen,
      }),
      width: drawerWidthClosed,
      [theme.breakpoints.up('sm')]: {
        width: theme.spacing(9),
      },
    }),
  },
}));

const mdTheme = createTheme({
  typography: {
    fontFamily:
      "'SF Pro Display','SF Compact Display',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif,'Apple Color Emoji','Segoe UI Emoji','Segoe UI Symbol'",
  },
});

type DashboardContentProps = {
  title: string;
  navbarColor: string;
  children?: React.ReactElement | React.ReactElement[];
};

function Content({ title, navbarColor, children }: DashboardContentProps) {
  const [open, setOpen] = React.useState(false);
  const toggleDrawer = () => {
    setOpen(!open);
  };

  return (
    <ThemeProvider theme={mdTheme}>
      <Box sx={{ display: 'flex' }}>
        <CssBaseline />
        <AppBar
          position="absolute"
          open={open}
          elevation={0}
          sx={{
            borderBottom: 1,
            borderColor: 'grey.300',
          }}
        >
          <Toolbar
            sx={{
              pr: '24px', // keep right padding when drawer closed
              backgroundColor: 'white',
            }}
          >
            <Typography
              component="h1"
              variant="h6"
              gutterBottom
              sx={{
                fontWeight: '800',
                color: navbarColor || '#404040',
              }}
            >
              {title || 'dagu'}
            </Typography>
          </Toolbar>
        </AppBar>
        <Drawer variant="permanent" open={open}>
          <Toolbar
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <IconButton onClick={toggleDrawer}>
              <img
                src={icon}
                alt="dagu"
                width={64}
                style={{
                  maxWidth: '64px',
                }}
              />
            </IconButton>
          </Toolbar>
          <List
            component="nav"
            sx={{
              pl: '6px',
            }}
          >
            {mainListItems}
          </List>
        </Drawer>
        <Box
          component="main"
          sx={{
            backgroundColor: 'white',
            flexGrow: 1,
            height: '100vh',
            overflow: 'auto',
          }}
        >
          <Toolbar />
          <Grid container sx={{ mt: 4, mb: 4 }}>
            {children}
          </Grid>
        </Box>
      </Box>
    </ThemeProvider>
  );
}

type DashboardProps = DashboardContentProps;

export default function Layout({ children, ...props }: DashboardProps) {
  return <Content {...props}>{children}</Content>;
}
