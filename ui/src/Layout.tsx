import * as React from 'react';
import { styled, createTheme, ThemeProvider } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import MuiDrawer from '@mui/material/Drawer';
import Box from '@mui/material/Box';
import MuiAppBar, { AppBarProps as MuiAppBarProps } from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import List from '@mui/material/List';
import Typography from '@mui/material/Typography';
import { mainListItems } from './menu';
import { Grid, IconButton } from '@mui/material';
import icon from '../../assets/images/dagu.png';
import { AppBarContext } from './contexts/AppBarContext';

const drawerWidthClosed = 64;
const drawerWidth = 240;

interface AppBarProps extends MuiAppBarProps {
  open?: boolean;
}

const AppBar = styled(MuiAppBar, {
  shouldForwardProp: (prop) => prop !== 'open',
})<AppBarProps>(({ theme, open }) => ({
  zIndex: theme.zIndex.drawer - 1,
  transition: theme.transitions.create(['width', 'margin', 'border'], {
    easing: theme.transitions.easing.sharp,
    duration: theme.transitions.duration.leavingScreen,
  }),
  width: '100%',
  ...(open && {
    transition: theme.transitions.create(['width', 'margin', 'border'], {
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
  palette: {
    primary: {
      main: '#485fc7',
    },
  },
});

type DashboardContentProps = {
  title: string;
  navbarColor: string;
  version: string;
  children?: React.ReactElement | React.ReactElement[];
};

function Content({
  title,
  navbarColor,
  version,
  children,
}: DashboardContentProps) {
  const [open, setOpen] = React.useState(false);
  const toggleDrawer = () => {
    setOpen(!open);
  };
  const [scrolled, setScrolled] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);
  const gradientColor = navbarColor || '#485fc7';

  return (
    <ThemeProvider theme={mdTheme}>
      <Box sx={{ display: 'flex', flexDirection: 'row', width: '100vw' }}>
        <CssBaseline />
        <Drawer variant="permanent" open={open}>
          <Box
            sx={{
              background: `linear-gradient(0deg, #fff 0%, ${gradientColor} 70%, ${gradientColor} 100%);`,
              height: '100%',
            }}
          >
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
            <Box
              sx={{
                display: 'flex',
                justifyContent: 'center',
                alignItems: 'center',
                color: '#fff',
                overflowWrap: 'break-word',
              }}
            >
              {version ? `v${version}` : "dev"}
            </Box>
            <List
              component="nav"
              sx={{
                pl: '6px',
              }}
            >
              {mainListItems}
            </List>
          </Box>
        </Drawer>
        <Box
          component="main"
          sx={{
            display: 'flex',
            flexDirection: 'column',
            backgroundColor: 'white',
            height: '100vh',
            width: '100%',
            maxWidth: '100%',
            overflow: 'auto',
          }}
        >
          <AppBar
            open={open}
            elevation={0}
            sx={{
              width: '100%',
              backgroundColor: 'white',
              borderBottom: scrolled ? 1 : 0,
              borderColor: 'grey.300',
              pr: 2,
              position: 'relative',
              display: 'block',
            }}
          >
            <Toolbar
              sx={{
                width: '100%',
                backgroundColor: 'white',
                display: 'flex',
                direction: 'row',
                justifyContent: 'space-between',
                alignItems: 'center',
                flex: 1,
              }}
            >
              <AppBarContext.Consumer>
                {(context) => (
                  <NavBarTitleText visible={scrolled}>
                    {context.title}
                  </NavBarTitleText>
                )}
              </AppBarContext.Consumer>
              <NavBarTitleText>{title || 'dagu'}</NavBarTitleText>
            </Toolbar>
          </AppBar>
          <Grid
            container
            ref={containerRef}
            sx={{
              flex: 1,
              pt: 2,
              pb: 4,
              overflow: 'auto',
            }}
            onScroll={() => {
              const curr = containerRef.current;
              if (curr) {
                setScrolled(curr.scrollTop > 54);
              }
            }}
          >
            {children}
          </Grid>
        </Box>
      </Box>
    </ThemeProvider>
  );
}

type NavBarTitleTextProps = {
  children: string;
  visible?: boolean;
};

const NavBarTitleText = ({
  children,
  visible = true,
}: NavBarTitleTextProps) => (
  <Typography
    component="h1"
    variant="h6"
    gutterBottom
    sx={{
      fontWeight: '800',
      color: '#404040',
      opacity: visible ? 1 : 0,
      transition: 'opacity 0.2s',
    }}
  >
    {children}
  </Typography>
);

type DashboardProps = DashboardContentProps;

export default function Layout({ children, ...props }: DashboardProps) {
  return <Content {...props}>{children}</Content>;
}
