const HtmlWebpackPlugin = require("html-webpack-plugin");
const { merge } = require("webpack-merge");
const common = require("./webpack.common.js");
const path = require("path");

module.exports = merge(common, {
  mode: "development",
  devtool: "eval-source-map",
  profile: true,
  devServer: {
    historyApiFallback: true,
    port: 8081,
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: [
          {
            loader: "esbuild-loader",
            options: {
              loader: "tsx",
              target: "es2015",
            },
          },
        ],
        include: path.resolve(__dirname, "src"),
        exclude: path.resolve(__dirname, "node_modules"),
      },
    ],
  },
  plugins: [
    new HtmlWebpackPlugin({
      template: "index.html",
    }),
  ],
  optimization: {
    removeAvailableModules: false,
    removeEmptyChunks: false,
    splitChunks: false,
  },
  output: {
    filename: 'bundle.js',
    pathinfo: false,
    path: path.resolve(__dirname, 'dist'),
    publicPath: '/',
    clean: true,
  },
});
