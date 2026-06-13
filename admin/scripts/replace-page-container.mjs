import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const pagesDir = path.join(__dirname, '../src/pages');

function walk(dir, files = []) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p, files);
    else if (ent.name.endsWith('.tsx')) files.push(p);
  }
  return files;
}

const files = walk(pagesDir).filter((f) => !f.includes(`${path.sep}Settings${path.sep}Platforms${path.sep}`));

for (const file of files) {
  let src = fs.readFileSync(file, 'utf8');
  if (!/<PageContainer[\s>]/.test(src)) continue;

  src = src.replace(
    /<PageContainer\s*\n\s*header=\{\{\s*\n\s*title:\s*([^,]+),\s*\n\s*subTitle:\s*([^,]+),?\s*\n\s*\}\}/g,
    '<TmPageContainer\n      title={$1}\n      subTitle={$2}',
  );

  src = src.replace(/<PageContainer/g, '<TmPageContainer');
  src = src.replace(/<\/PageContainer>/g, '</TmPageContainer>');

  src = src.replace(/import\s*\{([^}]*)\}\s*from\s*'@ant-design\/pro-components';/g, (m, inner) => {
    const parts = inner
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
      .filter((p) => p !== 'PageContainer');
    if (parts.length === 0) return '';
    return `import { ${parts.join(', ')} } from '@ant-design/pro-components';`;
  });

  src = src.replace(/\nimport\s*\{\s*\}\s*from\s*'@ant-design\/pro-components';\n/g, '\n');

  if (!src.includes('TmPageContainer')) continue;

  if (src.includes("from '@/components/ui'")) {
    src = src.replace(/import\s*\{([^}]*)\}\s*from\s*'@\/components\/ui';/, (m, inner) => {
      const parts = inner
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
      if (!parts.includes('TmPageContainer')) parts.unshift('TmPageContainer');
      return `import { ${parts.join(', ')} } from '@/components/ui';`;
    });
  } else {
    const uiImport = "import { TmPageContainer } from '@/components/ui';";
    const match = src.match(/^import .+ from '@ant-design\/pro-components';$/m);
    if (match) {
      const idx = src.indexOf(match[0]) + match[0].length;
      src = `${src.slice(0, idx)}\n${uiImport}${src.slice(idx)}`;
    } else {
      const firstImport = src.indexOf('import ');
      const lineEnd = src.indexOf('\n', firstImport);
      src = `${src.slice(0, lineEnd + 1)}${uiImport}\n${src.slice(lineEnd + 1)}`;
    }
  }

  fs.writeFileSync(file, src);
  console.log('updated', path.relative(pagesDir, file));
}
