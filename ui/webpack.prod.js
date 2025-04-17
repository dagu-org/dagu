const { merge } = require('webpack-merge');
const common = require('./webpack.common.js');
const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');

module.exports = merge(common, {
  mode: 'production',
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: [
          {
            loader: 'ts-loader',
          },
        ],
        include: path.resolve(__dirname, 'src'),
        exclude: [path.resolve(__dirname, 'node_modules')],
      },
    ],
  },
  output: {
    filename: 'bundle.js?v=0.0.0', // Add version query to prevent caching issues
    path: path.resolve(__dirname, 'dist'),
    publicPath: 'auto',
    clean: true,
  },
});
