module.exports = {
  filenameHashing: true,
  productionSourceMap: false,
  configureWebpack: {
    output: {
      filename: 'js/[name].[hash:8].js',
      chunkFilename: 'js/[name].[hash:8].js',
    },
  },
  css: {
    extract: {
      filename: 'css/[name].[hash:8].css',
      chunkFilename: 'css/[name].[hash:8].css',
    },
  },
}
