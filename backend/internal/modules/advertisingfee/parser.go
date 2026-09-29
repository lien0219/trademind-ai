package advertisingfee

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

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
		key := row.SpendDate + "\x1f" + row.Currency
		if previous, ok := seen[key]; row.SpendDate != "" && row.Currency != "" && ok {
			rowIssues = append(rowIssues, ValidationIssue{Line: line, Field: CSVHeaders[0], Code: "duplicate_spend_day", Message: fmt.Sprintf("店铺日期和币种与第 %d 行重复", previous)})
		} else if row.SpendDate != "" && row.Currency != "" {
			seen[key] = line
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
		issues = append(issues, ValidationIssue{Line: 2, Code: "empty_data", Message: "CSV 不包含广告费用"})
	}
	return rows, issues
}

func parseCSVRow(line int, record []string) (CSVRow, []ValidationIssue) {
	row := CSVRow{
		Line: line, SpendDate: strings.TrimSpace(record[0]), Currency: strings.ToUpper(strings.TrimSpace(record[1])),
		SettlementCoverage: strings.ToLower(strings.TrimSpace(record[3])),
	}
	issues := make([]ValidationIssue, 0)
	if parsed, err := time.Parse("2006-01-02", row.SpendDate); err != nil || parsed.Format("2006-01-02") != row.SpendDate {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[0], Code: "invalid_spend_date", Message: "费用日期必须为 YYYY-MM-DD"})
	}
	if !validCurrency(row.Currency) {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[1], Code: "invalid_currency", Message: "币种必须为 3 位大写字母"})
	}
	amount, err := strconv.ParseInt(strings.TrimSpace(record[2]), 10, 64)
	if err != nil || amount <= 0 || amount > MaxJSONSafeInt64 {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[2], Code: "invalid_spend_minor", Message: "广告费用必须为 JavaScript 安全整数范围内的正数最小货币单位"})
	} else {
		row.SpendMinor = amount
	}
	if !validCoverage(row.SettlementCoverage) {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[3], Code: "invalid_settlement_coverage", Message: "结算覆盖必须为 excluded、included 或 unknown"})
	}
	return row, issues
}

func hashRow(row CSVRow) string {
	canonical := strings.Join([]string{row.SpendDate, row.Currency, strconv.FormatInt(row.SpendMinor, 10), row.SettlementCoverage}, "\x1f")
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

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func validCoverage(value string) bool {
	return value == CoverageExcluded || value == CoverageIncluded || value == CoverageUnknown
}
