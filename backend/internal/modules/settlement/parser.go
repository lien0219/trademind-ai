package settlement

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const maxJSONSafeInteger int64 = 9007199254740991

func fileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func parseCSV(data []byte) ([]CSVRow, []ValidationIssue) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = false

	header, err := reader.Read()
	if err != nil {
		message := "CSV 文件为空"
		if !errors.Is(err, io.EOF) {
			message = "CSV 表头无法解析"
		}
		return []CSVRow{}, []ValidationIssue{{Line: 1, Code: "invalid_header", Message: message}}
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	if !equalStrings(header, CSVHeaders) {
		return []CSVRow{}, []ValidationIssue{{Line: 1, Code: "invalid_header", Message: "CSV 表头必须严格匹配模板字段和顺序"}}
	}

	rows := make([]CSVRow, 0)
	issues := make([]ValidationIssue, 0)
	seen := make(map[string]int)
	dataRows := 0
	for line := 2; ; line++ {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			issues = append(issues, ValidationIssue{Line: line, Code: "invalid_csv", Message: "CSV 行无法解析"})
			continue
		}
		dataRows++
		if dataRows > MaxCSVRows {
			issues = append(issues, ValidationIssue{Line: line, Code: "too_many_rows", Message: fmt.Sprintf("单次最多导入 %d 行", MaxCSVRows)})
			break
		}
		if len(record) != len(CSVHeaders) {
			issues = append(issues, ValidationIssue{Line: line, Code: "invalid_column_count", Message: "字段数量与模板不一致"})
			continue
		}
		row, rowIssues := parseCSVRow(line, record)
		if previousLine, ok := seen[row.ExternalTransactionID]; row.ExternalTransactionID != "" && ok {
			rowIssues = append(rowIssues, ValidationIssue{Line: line, Field: CSVHeaders[0], Code: "duplicate_external_transaction", Message: fmt.Sprintf("外部交易号与第 %d 行重复", previousLine)})
		} else if row.ExternalTransactionID != "" {
			seen[row.ExternalTransactionID] = line
		}
		if len(rowIssues) > 0 {
			issues = append(issues, rowIssues...)
			continue
		}
		row.RowHash = hashRow(row)
		row.Disposition = DispositionNew
		rows = append(rows, row)
	}
	if len(rows) == 0 && len(issues) == 0 {
		issues = append(issues, ValidationIssue{Line: 2, Code: "empty_data", Message: "CSV 不包含结算明细"})
	}
	return rows, issues
}

func parseCSVRow(line int, record []string) (CSVRow, []ValidationIssue) {
	row := CSVRow{
		Line:                  line,
		ExternalTransactionID: strings.TrimSpace(record[0]),
		OrderNo:               strings.TrimSpace(record[1]),
		Currency:              strings.ToUpper(strings.TrimSpace(record[2])),
	}
	issues := make([]ValidationIssue, 0)
	if row.ExternalTransactionID == "" || len(row.ExternalTransactionID) > 255 {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[0], Code: "invalid_external_transaction_id", Message: "外部交易号不能为空且最多 255 个字符"})
	}
	if row.OrderNo == "" || len(row.OrderNo) > 128 {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[1], Code: "invalid_order_no", Message: "订单号不能为空且最多 128 个字符"})
	}
	if !validCurrency(row.Currency) {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[2], Code: "invalid_currency", Message: "币种必须为 3 位大写字母"})
	}
	amountTargets := []struct {
		field string
		raw   string
		out   *int64
	}{
		{CSVHeaders[3], record[3], &row.OrderGrossMinor},
		{CSVHeaders[4], record[4], &row.PlatformFeeMinor},
		{CSVHeaders[5], record[5], &row.SettlementAmountMinor},
	}
	amountsValid := true
	for _, target := range amountTargets {
		value, parseErr := strconv.ParseInt(strings.TrimSpace(target.raw), 10, 64)
		if parseErr != nil || !jsonSafeInteger(value) {
			amountsValid = false
			issues = append(issues, ValidationIssue{Line: line, Field: target.field, Code: "invalid_minor_amount", Message: "金额必须为 JavaScript 安全整数范围内的最小货币单位"})
			continue
		}
		*target.out = value
	}
	if row.OrderGrossMinor < 0 {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[3], Code: "negative_order_gross", Message: "订单交易总额不能为负数；调整行请填写 0"})
	}
	settledAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(record[6]))
	if parseErr != nil {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[6], Code: "invalid_settled_at", Message: "结算时间必须为 RFC3339 格式"})
	} else {
		row.SettledAt = settledAt.UTC()
	}
	if amountsValid {
		if expected, ok := safeSubtract(row.OrderGrossMinor, row.PlatformFeeMinor); !ok || expected != row.SettlementAmountMinor {
			issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[5], Code: "settlement_amount_mismatch", Message: "结算金额必须等于订单交易总额减平台费用"})
		}
	}
	return row, issues
}

func hashRow(row CSVRow) string {
	canonical := strings.Join([]string{
		row.ExternalTransactionID,
		row.OrderNo,
		row.Currency,
		strconv.FormatInt(row.OrderGrossMinor, 10),
		strconv.FormatInt(row.PlatformFeeMinor, 10),
		strconv.FormatInt(row.SettlementAmountMinor, 10),
		row.SettledAt.UTC().Format(time.RFC3339Nano),
	}, "\x1f")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func jsonSafeInteger(value int64) bool {
	return value >= -maxJSONSafeInteger && value <= maxJSONSafeInteger
}

func safeSubtract(left, right int64) (int64, bool) {
	if right == math.MinInt64 {
		return 0, false
	}
	return safeAdd(left, -right)
}

func safeAdd(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, false
	}
	return left + right, true
}
