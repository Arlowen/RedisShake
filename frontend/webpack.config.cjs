const path = require('node:path')
const HtmlWebpackPlugin = require('html-webpack-plugin')

module.exports = (_environment, argv) => ({
  entry: './src/main.tsx',
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: 'assets/[name].[contenthash:8].js',
    chunkFilename: 'assets/[name].[contenthash:8].js',
    assetModuleFilename: 'assets/[name].[contenthash:8][ext]',
    publicPath: '/',
    clean: true,
  },
  devtool: argv.mode === 'production' ? false : 'eval-cheap-module-source-map',
  resolve: {
    extensions: ['.tsx', '.ts', '.jsx', '.js'],
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  module: {
    rules: [
      {
        test: /\.[jt]sx?$/,
        exclude: /node_modules/,
        use: 'babel-loader',
      },
      {
        test: /\.css$/i,
        use: ['style-loader', 'css-loader'],
      },
      {
        test: /\.(woff2?|eot|ttf|otf)$/i,
        type: 'asset/resource',
      },
    ],
  },
  plugins: [new HtmlWebpackPlugin({ template: './index.html' })],
  optimization: {
    runtimeChunk: 'single',
    splitChunks: { chunks: 'all' },
  },
  performance: {
    maxAssetSize: 1024 * 1024,
    maxEntrypointSize: 1200 * 1024,
  },
  devServer: {
    host: '127.0.0.1',
    port: 5173,
    hot: true,
    historyApiFallback: true,
    proxy: [
      {
        context: ['/api', '/healthz', '/readyz'],
        target: process.env.REDISSHAKE_API_PROXY || 'http://127.0.0.1:8080',
      },
    ],
  },
})
