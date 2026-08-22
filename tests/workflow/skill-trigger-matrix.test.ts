import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { selectAffectedSuites } from '../../scripts/testing/test-affected.mjs';
import { coreSkills, validateSkillTriggers } from '../../scripts/workflow/check-skill-triggers.mjs';
import matrix from './skill-trigger-matrix.json';

const root = path.resolve(__dirname, '..', '..');
const scenarios = new Map(matrix.scenarios.map((scenario) => [scenario.id, scenario]));

function skillPath(skill: string) {
  return path.join(root, '.agents', 'skills', skill, 'SKILL.md');
}

function scenario(id: string) {
  const item = scenarios.get(id);
  if (!item) throw new Error(`Missing scenario ${id}`);
  return item;
}

function conflictFixture(skill = 'code-quality') {
  return {
    version: 1,
    scenarios: [
      {
        id: 'small-admin-ui',
        description: 'fixture conflict',
        files: ['admin/src/pages/Dashboard/index.tsx'],
        expectedSkills: ['frontend-design', 'frontend-unit-testing', 'admin-e2e-testing', 'code-quality', 'project-testing', skill],
        forbiddenSkills: [skill],
        expectedChecks: ['test:frontend', 'quality:affected', 'test:affected'],
        forbiddenChecks: [],
        depth: 'light',
        reason: 'fixture',
      },
    ],
  };
}

describe('workflow skill trigger matrix', () => {
  it('all Skill files exist', () => {
    for (const skill of coreSkills) {
      expect(existsSync(skillPath(skill)), skill).toBe(true);
    }
  });

  it('AGENTS.md references core Skill files', () => {
    const agents = readFileSync(path.join(root, 'AGENTS.md'), 'utf8');
    for (const skill of coreSkills) {
      expect(agents).toContain(`.agents/skills/${skill}/SKILL.md`);
    }
  });

  it('each expected Skill has a Cursor rule or AGENTS entry', () => {
    const result = validateSkillTriggers();
    expect(result.underTriggered).toEqual([]);
  });

  it('small-admin-ui does not trigger modular deep review', () => {
    const item = scenario('small-admin-ui');
    expect(item.depth).toBe('light');
    expect(item.expectedSkills).not.toContain('modular-architecture');
    expect(item.forbiddenSkills).toContain('modular-architecture');
    expect(item.expectedChecks.some((check) => check.startsWith('architecture:'))).toBe(false);
  });

  it('admin-interaction-bug triggers UI, E2E, code quality, and testing', () => {
    const item = scenario('admin-interaction-bug');
    expect(item.expectedSkills).toEqual(expect.arrayContaining(['frontend-design', 'frontend-unit-testing', 'admin-e2e-testing', 'code-quality', 'project-testing']));
  });

  it('admin-api-envelope triggers contract, backend, frontend, and E2E checks', () => {
    const item = scenario('admin-api-envelope');
    expect(item.expectedSkills).toEqual(expect.arrayContaining(['api-contract-testing', 'backend-testing', 'frontend-unit-testing', 'admin-e2e-testing']));
    expect(item.expectedChecks).toEqual(expect.arrayContaining(['test:contracts', 'test:backend', 'test:frontend', 'test:e2e:smoke']));
  });

  it('collector pure function does not trigger Admin E2E', () => {
    const item = scenario('collector-pure-function');
    expect(item.expectedSkills).not.toContain('admin-e2e-testing');
    expect(item.forbiddenSkills).toContain('admin-e2e-testing');
    expect(item.forbiddenChecks).toContain('test:e2e:smoke');
  });

  it('backend repository triggers architecture, backend, database, and code quality', () => {
    const item = scenario('backend-repository');
    expect(item.depth).toBe('deep');
    expect(item.expectedSkills).toEqual(expect.arrayContaining(['modular-architecture', 'backend-testing', 'code-quality']));
    expect(item.expectedChecks).toEqual(expect.arrayContaining(['architecture:affected', 'test:backend', 'test:db', 'quality:affected']));
  });

  it('migration triggers architecture, database, and contract checks', () => {
    const item = scenario('migration-change');
    expect(item.expectedSkills).toEqual(expect.arrayContaining(['modular-architecture', 'backend-testing', 'api-contract-testing']));
    expect(item.expectedChecks).toEqual(expect.arrayContaining(['architecture:affected', 'test:db', 'test:contracts']));
  });

  it('shared type triggers architecture, typecheck-adjacent frontend, and contract checks', () => {
    const item = scenario('shared-type-change');
    expect(item.expectedSkills).toEqual(expect.arrayContaining(['modular-architecture', 'api-contract-testing', 'frontend-unit-testing']));
    expect(item.expectedChecks).toEqual(expect.arrayContaining(['architecture:affected', 'test:contracts', 'test:frontend']));
  });

  it('new adapter triggers architecture deep review', () => {
    const item = scenario('new-platform-adapter');
    expect(item.depth).toBe('deep');
    expect(item.expectedSkills).toContain('modular-architecture');
    expect(item.forbiddenChecks).toContain('真实平台写请求');
  });

  it('new worker triggers architecture, queue, and idempotency-related checks', () => {
    const item = scenario('new-worker');
    expect(item.depth).toBe('deep');
    expect(item.expectedSkills).toEqual(expect.arrayContaining(['modular-architecture', 'backend-testing']));
    expect(item.expectedChecks).toContain('test:redis');
    expect(item.forbiddenChecks).toContain('无限重试');
  });

  it('documentation-only does not run full tests or E2E', () => {
    const item = scenario('documentation-only');
    expect(item.depth).toBe('light');
    expect(item.expectedChecks).toEqual(['quality:sensitive']);
    expect(item.forbiddenChecks).toContain('test:e2e:smoke');
  });

  it('architecture config triggers architecture test and check commands', () => {
    const item = scenario('architecture-config-change');
    expect(item.expectedSkills).toContain('modular-architecture');
    expect(item.expectedChecks).toEqual(expect.arrayContaining(['architecture:test', 'architecture:check', 'architecture:affected']));
  });

  it('orchestration scripts have no affected-command cycles', () => {
    const result = validateSkillTriggers();
    expect(result.affectedCycles).toEqual([]);
    expect(result.affectedGraph['quality:affected']).toContain('architecture:affected');
    expect(result.affectedGraph['architecture:affected']).not.toContain('quality:affected');
    expect(result.affectedGraph['test:affected']).not.toContain('quality:affected');
  });

  it('affected test selection can exclude an E2E smoke suite already run by quality checks', () => {
    expect([...selectAffectedSuites(['admin/e2e/specs/warehouse-ledger.spec.ts'])]).toEqual(['e2e-smoke']);
    expect([...selectAffectedSuites(['admin/e2e/specs/warehouse-ledger.spec.ts'], { skip: ['e2e-smoke'] })]).toEqual([]);
    expect([...selectAffectedSuites(['admin/src/pages/Inventory/WarehouseLedger/index.tsx', 'admin/e2e/specs/warehouse-ledger.spec.ts'], { skip: ['e2e-smoke'] })]).toEqual(['frontend']);
  });

  it('Skill path missing from AGENTS or rules fails validation', () => {
    const valid = validateSkillTriggers();
    expect(valid.failures.join('\n')).not.toContain('missing skill file');

    const broken = validateSkillTriggers({
      matrix: {
        version: 1,
        scenarios: matrix.scenarios.map((item) =>
          item.id === 'small-admin-ui'
            ? {
                ...item,
                expectedSkills: ['missing-skill', 'frontend-design', 'frontend-unit-testing', 'admin-e2e-testing', 'code-quality', 'project-testing'],
              }
            : item,
        ),
      },
    });
    expect(broken.failures.join('\n')).toContain('missing-skill');
  });

  it('expected and forbidden conflicts fail validation', () => {
    const result = validateSkillTriggers({ matrix: conflictFixture() });
    expect(result.failures.join('\n')).toContain('is both expected and forbidden');
  });

  it('current matrix passes static workflow validation', () => {
    const result = validateSkillTriggers();
    expect(result.failures).toEqual([]);
    expect(result.scenarioTotal).toBeGreaterThanOrEqual(14);
    expect(result.failedScenarios).toBe(0);
  });
});
