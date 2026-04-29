import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

const frontendRoot = new URL('..', import.meta.url).pathname
const pagesDir = join(frontendRoot, 'src/pages')
const ignoredPages = new Set(['login'])
const ignoredApiScopePages = new Set(['audit'])
const failures = []

function read(relativePath) {
  return readFileSync(join(frontendRoot, relativePath), 'utf8')
}

function pushFailure(message) {
  failures.push(message)
}

function extractNavPaths(taxonomySource) {
  const matches = [...taxonomySource.matchAll(/key:\s*'\/(?<path>[^']*)'/g)]
  return new Set(matches.map((match) => match.groups.path || '/'))
}

function extractRouteGuardPaths(routerSource) {
  const matches = [...routerSource.matchAll(/routeGuard\('(?<path>[^']+)'/g)]
  return new Set(matches.map((match) => match.groups.path))
}

function extractLazyPageNames(routerSource) {
  const matches = [...routerSource.matchAll(/const\s+(?<name>\w+Page)\s*=\s*lazy\(\(\)\s*=>\s*import\('\.\.\/pages\/(?<page>[^']+)'\)\)/g)]
  return matches
    .map((match) => ({ name: match.groups.name, page: match.groups.page }))
    .filter((item) => item.page !== 'login')
}

for (const pageName of readdirSync(pagesDir).sort()) {
  if (ignoredPages.has(pageName)) continue
  const file = join(pagesDir, pageName, 'index.tsx')
  try {
    if (!statSync(file).isFile()) continue
  } catch {
    continue
  }

  const source = readFileSync(file, 'utf8')
  if (!source.includes('ScopeNotice')) {
    pushFailure(`${file}: missing ScopeNotice`)
  }
  if (source.includes('apiClient.') && !ignoredApiScopePages.has(pageName) && !source.includes('usePlatformScope')) {
    pushFailure(`${file}: apiClient page missing usePlatformScope`)
  }
}

const taxonomy = read('src/config/permission-taxonomy.ts')
const router = read('src/app/router.tsx')
const layout = read('src/layouts/AdminLayout.tsx')
const permissionMeta = read('src/config/permission-meta.ts')

if (!taxonomy.includes('allowedLevels')) {
  pushFailure('permission-taxonomy.ts: missing allowedLevels metadata')
}
if (!router.includes('allowedByLevel') || !router.includes('routeGuard(')) {
  pushFailure('router.tsx: route guard does not enforce allowedLevels')
}
if (!layout.includes('allowedLevels') || !layout.includes('scopeLevel')) {
  pushFailure('AdminLayout.tsx: menu does not enforce allowedLevels')
}
if (!permissionMeta.includes('missingMetaCodes')) {
  pushFailure('permission-meta.ts: missing permission metadata self-check')
}

const navPaths = extractNavPaths(taxonomy)
const guardedPaths = extractRouteGuardPaths(router)
for (const path of navPaths) {
  if (!guardedPaths.has(path)) {
    pushFailure(`router.tsx: nav path ${path} is not protected by routeGuard`)
  }
}

for (const { name, page } of extractLazyPageNames(router)) {
  const pageRoute = page === 'dashboard' ? '/' : page
  if (!router.includes(`routeGuard('${pageRoute}'`) || !router.includes(`<${name} />`)) {
    pushFailure(`router.tsx: lazy page ${page} is not wired through routeGuard`)
  }
}

if (!taxonomy.includes("key: '/audit'") || !taxonomy.includes("allowedLevels: ['platform']")) {
  pushFailure('permission-taxonomy.ts: audit route must remain platform-only')
}
for (const platformTenantBrandPath of ['/tenant-brand', '/platform-config-center', '/agent-game-access', '/rbac']) {
  const pathIndex = taxonomy.indexOf(`key: '${platformTenantBrandPath}'`)
  if (pathIndex < 0) {
    pushFailure(`permission-taxonomy.ts: missing ${platformTenantBrandPath}`)
    continue
  }
  const snippet = taxonomy.slice(pathIndex, pathIndex + 260)
  if (!snippet.includes("allowedLevels: ['platform', 'tenant', 'brand']")) {
    pushFailure(`permission-taxonomy.ts: ${platformTenantBrandPath} must be limited to platform/tenant/brand`)
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log('SaaS scope frontend checks passed')
