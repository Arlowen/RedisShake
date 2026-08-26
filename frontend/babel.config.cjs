module.exports = (api) => {
  const test = api.env('test')
  return {
    presets: [
      ['@babel/preset-env', { targets: test ? { node: 'current' } : { browsers: 'defaults' }, modules: test ? 'commonjs' : false }],
      ['@babel/preset-react', { runtime: 'automatic', development: false }],
      '@babel/preset-typescript',
    ],
  }
}
