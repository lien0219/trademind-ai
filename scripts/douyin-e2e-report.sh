#!/usr/bin/env bash
# Aggregate Douyin E2E JSON reports into markdown (DOUYIN_E2E_REPORT_TEMPLATE fields).
set -euo pipefail

REPORT_DIR="${DOUYIN_E2E_REPORT_DIR:-./tmp/douyin-e2e}"
OUT_MD="${DOUYIN_E2E_REPORT_MD:-$REPORT_DIR/douyin-e2e-report.md}"
GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

blocked="是"
release_status="Release Candidate"
gray_ok="否"
rollback_drill="environment_simulation_only"
ci_race="see .github/workflows/go.yml backend-race job"

if [ -d "$REPORT_DIR" ]; then
  if ls "$REPORT_DIR"/preflight-*.json >/dev/null 2>&1; then
    latest_preflight="$(ls -t "$REPORT_DIR"/preflight-*.json | head -1)"
    if grep -q '"blockedByRealCredentials"[[:space:]]*:[[:space:]]*false' "$latest_preflight" 2>/dev/null; then
      blocked="否"
    fi
  fi
fi

cat > "$OUT_MD" <<EOF
# 抖店 E2E 验收报告（脚本生成）

> 生成时间（UTC）：$TS  
> Git SHA：$GIT_SHA  
> 发布状态：$release_status  
> \`blocked_by_real_credentials\`：$blocked

## 发布结论（Phase 10.4）

| 字段 | 值 |
| --- | --- |
| release_candidate_status | $release_status |
| real_e2e_status | $([ "$blocked" = "是" ] && echo blocked_by_real_credentials || echo passed_or_partial) |
| gray_release_approved | $gray_ok |
| rollback_drill_status | $rollback_drill |
| ci_backend_race | $ci_race |
| production_available | 否 |

## 脚本工件

目录：\`$REPORT_DIR\`

| 脚本 | 用途 |
| --- | --- |
| \`scripts/douyin-e2e-preflight.sh\` | 健康检查 + 生产预检 + 运行状态 |
| \`scripts/douyin-e2e-readonly.sh\` | 只读链路探针 |
| \`scripts/douyin-e2e-write.sh\` | 写链路（需 \`ALLOW_DOUYIN_WRITE_TEST=true\`） |

## 下一步

- 配置真实抖店 App Key / Secret 并完成 OAuth 后重跑脚本
- 填写完整模板：[\`docs/DOUYIN_E2E_REPORT_TEMPLATE.md\`](../docs/DOUYIN_E2E_REPORT_TEMPLATE.md)
- 灰度门禁：[\`docs/DOUYIN_RELEASE_GATE.md\`](../docs/DOUYIN_RELEASE_GATE.md)

EOF

echo "report: $OUT_MD"
if [ "$blocked" = "是" ]; then
  echo "blocked_by_real_credentials" >&2
  exit 3
fi
echo "ok: report generated"
