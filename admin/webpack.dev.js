const HtmlWebpackPlugin = require("html-webpack-plugin");
const webpack = require("webpack");
const { merge } = require("webpack-merge");
const common = require("./webpack.common.js");

module.exports = merge(common, {
  mode: "development",
  devtool: "inline-source-map",
  devServer: {
    historyApiFallback: true,
  },
  plugins: [
    new webpack.DefinePlugin({
      API_URL: JSON.stringify("http://127.0.0.1:8080"),
    }),
    new HtmlWebpackPlugin({
      template: "index.html",
    }),
  ],
});
