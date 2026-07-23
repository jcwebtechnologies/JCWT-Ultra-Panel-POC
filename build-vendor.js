/**
 * build-vendor.js
 * Bundles VueFinder + Vue 3 + all dependencies into two self-contained files:
 *   - web/js/vendor/vuefinder.bundle.js  (ESM, no external imports)
 *   - web/css/vendor/vuefinder.css       (full stylesheet)
 */

import esbuild from 'esbuild';
import { copyFileSync, mkdirSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const outJsDir = resolve(__dirname, 'cmd/jcwt-panel/web/js/vendor');
const outCssDir = resolve(__dirname, 'cmd/jcwt-panel/web/css/vendor');

mkdirSync(outJsDir, { recursive: true });
mkdirSync(outCssDir, { recursive: true });

// Bundle VueFinder + Vue + all deps into one ESM file
await esbuild.build({
  entryPoints: ['node_modules/vuefinder/dist/vuefinder.js'],
  bundle: true,
  format: 'esm',
  target: 'es2020',
  outfile: `${outJsDir}/vuefinder.bundle.js`,
  alias: {
    vue: resolve(__dirname, 'node_modules/vue/dist/vue.esm-bundler.js'),
  },
  define: {
    'process.env.NODE_ENV': '"production"',
    '__DEV__': 'false',
    '__VUE_OPTIONS_API__': 'true',
    '__VUE_PROD_DEVTOOLS__': 'false',
    '__VUE_PROD_HYDRATION_MISMATCH_DETAILS__': 'false',
  },
  minify: false,
  treeShaking: true,
  platform: 'browser',
  logLevel: 'info',
});

// Copy VueFinder CSS
const vuefinderCssSrc = resolve(__dirname, 'node_modules/vuefinder/dist/vuefinder.css');
copyFileSync(vuefinderCssSrc, `${outCssDir}/vuefinder.css`);

console.log('✓ vuefinder.bundle.js built');
console.log('✓ vuefinder.css copied');
