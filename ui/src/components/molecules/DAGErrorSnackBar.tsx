import { Alert, AlertTitle, Box, Snackbar} from "@mui/material";
import React from "react";

type DAGErrorSnackBarProps = {
    open: boolean;
    setOpen: (open: boolean) => void;
    errors: string[];
};

const DAGErrorSnackBar = ({ open, setOpen, errors }: DAGErrorSnackBarProps) => {
    const handleClose = () => {
        setOpen(false);
    };

    return (
        <Snackbar
            anchorOrigin={{
                vertical: 'top',
                horizontal: 'center',
            }}
            security="error"
            open={open}
            autoHideDuration={6000}
            onClose={handleClose}
        >
            <Alert
                severity="error"
                sx={{
                    width: '20vw',
                }}
                onClose={handleClose}
            >
                <Box
                    sx={{
                        display: 'flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                    }}
                >
                    <AlertTitle
                        sx={{
                            color: '#FF4D4D',
                            fontSize: '1.5rem',
                        }}
                    >Error Detected</AlertTitle>
                    <Box
                        sx={{
                            color: '#FC7E7E',
                            fontSize: '1.2rem',
                        }}
                    >
                        Please check the following errors:
                    </Box>
                    {errors.map((error, index) => (
                        <Box key={index}
                            sx={{
                                color: '#FC7E7E',
                                fontSize: '1rem',
                                margin: '2px',
                            }}
                        >
                            {error}
                        </Box>
                    ))}
                </Box>
            </Alert>
        </Snackbar>
    );
}

export default DAGErrorSnackBar;