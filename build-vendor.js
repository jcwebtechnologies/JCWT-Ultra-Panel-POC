/**
 * build-vendor.js
 * Bundles VueFinder + all dependencies (except Vue) into a self-contained ESM file.
 * Vue is kept external and loaded separately so createApp/h are available.
 *
 * Output:
 *   cmd/jcwt-panel/web/js/vendor/vue.esm-browser.prod.js  (already present, unchanged)
 *   cmd/jcwt-panel/web/js/vendor/vuefinder.bundle.js       (VueFinder + all non-Vue deps)
 *   cmd/jcwt-panel/web/css/vendor/vuefinder.css
 *
 * Usage: node build-vendor.js
 */

import esbuild from 'esbuild';
import { copyFileSync, mkdirSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const outJsDir  = resolve(__dirname, 'cmd/jcwt-panel/web/js/vendor');
const outCssDir = resolve(__dirname, 'cmd/jcwt-panel/web/css/vendor');

mkdirSync(outJsDir,  { recursive: true });
mkdirSync(outCssDir, { recursive: true });

await esbuild.build({
  entryPoints: ['node_modules/vuefinder/dist/vuefinder.js'],
  bundle:      true,
  format:      'esm',
  target:      'es2020',
  outfile:     `${outJsDir}/vuefinder.bundle.js`,
  // Keep vue external — we import vue.esm-browser.prod.js separately in the browser
  external:    ['vue'],
  define: {
    'process.env.NODE_ENV':                        '"production"',
    '__DEV__':                                     'false',
    '__VUE_OPTIONS_API__':                         'true',
    '__VUE_PROD_DEVTOOLS__':                       'false',
    '__VUE_PROD_HYDRATION_MISMATCH_DETAILS__':     'false',
  },
  minify:       false,
  treeShaking:  true,
  platform:     'browser',
  logLevel:     'info',
});

// Copy VueFinder CSS
copyFileSync(
  resolve(__dirname, 'node_modules/vuefinder/dist/vuefinder.css'),
  `${outCssDir}/vuefinder.css`,
);

console.log('✓ vuefinder.bundle.js  (vue external)');
console.log('✓ vuefinder.css copied');
