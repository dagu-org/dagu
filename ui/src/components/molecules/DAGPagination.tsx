import { Box, Pagination } from "@mui/material";
import React from "react";

type DAGPaginationProps = {
    totalPages: number;
    page: number;
    pageChange: (page: number) => void;
};

const DAGPagination = ({ totalPages, page, pageChange }: DAGPaginationProps) => {
    const handleChange = (event: React.ChangeEvent<unknown>, value: number) => {
        pageChange(value);
    };

    return (
        <Box
            sx={{
                display: 'flex',
                flexDirection: 'row',
                alignItems: 'center',
                justifyContent: 'center',
                mt: 2,
            }}
        >
            <Pagination
                count={totalPages}
                page={page}
                onChange={handleChange}
            />
        </Box>
    );
}

export default DAGPagination;