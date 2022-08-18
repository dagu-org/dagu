const MonacoWebpackPlugin = require('monaco-editor-webpack-plugin');
const path = require("path");

module.exports = {
  entry: "./src/index.tsx",
  module: {
    rules: [
      {
        test: /\.css$/,
        use: ["style-loader", "css-loader"],
      },
      {
        test: /\.(png|jpg|gif)$/i,
        use: [
          {
            loader: "url-loader",
            options: {
              encoding: "base64",
            },
          },
        ],
      },
			{
				test: /\.ttf$/,
				type: 'asset/resource'
			}
    ],
  },
  plugins: [
    new MonacoWebpackPlugin(
      {
        languages: ["yaml"],
        features: ["find"]
      }
    ),
  ],
  resolve: {
    extensions: [".tsx", ".ts", ".js"],
  },
  output: {
    filename: "bundle.js",
    path: path.resolve(__dirname, "dist"),
    publicPath: "/",
    clean: true,
  },
};
