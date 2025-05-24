const { merge } = require('webpack-merge');
const common = require('./webpack.common.js');
const path = require('path');
const TerserPlugin = require('terser-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const CopyWebpackPlugin = require('copy-webpack-plugin');

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
  plugins: [
    new HtmlWebpackPlugin({
      template: 'index.html',
      favicon: 'favicon.ico',
    }),
    new CopyWebpackPlugin({
      patterns: [
        {
          from: 'favicon.ico',
          to: '',
        },
        {
          from: 'favicon.ico',
          to: 'favicon.ico',
        },
      ],
    }),
  ],
  output: {
    filename: 'bundle.js?v=0.0.0', // Add version query to prevent caching issues
    path: path.resolve(__dirname, 'dist'),
    publicPath: 'auto',
    clean: true,
  },
});
