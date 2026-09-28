package freightfee

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
	"unicode"
	"unicode/utf8"
)

func parseCSV(data []byte) ([]CSVRow, []ValidationIssue) {
	if !utf8.Valid(data) {
		return []CSVRow{}, []ValidationIssue{{Line: 1, Code: "invalid_encoding", Message: "CSV 必须使用 UTF-8 编码"}}
	}
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
	seenIDs := make(map[string]int)
	seenTracking := make(map[string]int)
	for line := 2; ; line++ {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			issues = append(issues, ValidationIssue{Line: line, Code: "invalid_csv", Message: "CSV 行无法解析"})
			continue
		}
		if line-1 > MaxCSVRows {
			issues = append(issues, ValidationIssue{Line: line, Code: "too_many_rows", Message: fmt.Sprintf("单次最多导入 %d 行", MaxCSVRows)})
			break
		}
		if len(record) != len(CSVHeaders) {
			issues = append(issues, ValidationIssue{Line: line, Code: "invalid_column_count", Message: "字段数量与模板不一致"})
			continue
		}
		row, rowIssues := parseCSVRow(line, record)
		idKey := normalizeCarrier(row.Carrier) + "\x1f" + row.ExternalLineID
		if row.ExternalLineID != "" {
			if previous, ok := seenIDs[idKey]; ok {
				rowIssues = append(rowIssues, ValidationIssue{Line: line, Field: CSVHeaders[0], Code: "duplicate_external_line", Message: fmt.Sprintf("承运商费用编号与第 %d 行重复", previous)})
			} else {
				seenIDs[idKey] = line
			}
		}
		trackingKey := normalizeCarrier(row.Carrier) + "\x1f" + row.TrackingNo
		if row.TrackingNo != "" {
			if previous, ok := seenTracking[trackingKey]; ok {
				rowIssues = append(rowIssues, ValidationIssue{Line: line, Field: CSVHeaders[2], Code: "duplicate_tracking_line", Message: fmt.Sprintf("同一承运商运单已出现在第 %d 行；每个包裹必须汇总为一条最终账单行", previous)})
			} else {
				seenTracking[trackingKey] = line
			}
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
		issues = append(issues, ValidationIssue{Line: 2, Code: "empty_data", Message: "CSV 不包含运费记录"})
	}
	return rows, issues
}

func parseCSVRow(line int, record []string) (CSVRow, []ValidationIssue) {
	row := CSVRow{
		Line: line, ExternalLineID: strings.TrimSpace(record[0]), Carrier: strings.TrimSpace(record[1]),
		TrackingNo: strings.TrimSpace(record[2]), Currency: strings.ToUpper(strings.TrimSpace(record[4])),
	}
	issues := make([]ValidationIssue, 0)
	if row.ExternalLineID == "" || len([]rune(row.ExternalLineID)) > 128 || containsControl(row.ExternalLineID) {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[0], Code: "invalid_external_line_id", Message: "承运商费用编号必填且不能超过 128 个字符"})
	}
	if row.Carrier == "" || len([]rune(row.Carrier)) > 128 || containsControl(row.Carrier) {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[1], Code: "invalid_carrier", Message: "承运商名称必填且不能超过 128 个字符"})
	}
	if row.TrackingNo == "" || len([]rune(row.TrackingNo)) > 255 || containsControl(row.TrackingNo) {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[2], Code: "invalid_tracking_no", Message: "运单号必填且不能超过 255 个字符"})
	}
	amount, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
	if err != nil || amount < 0 || amount > MaxJSONSafeInt64 {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[3], Code: "invalid_amount_minor", Message: "运费必须为 JavaScript 安全整数范围内的非负最小货币单位"})
	} else {
		row.AmountMinor = amount
	}
	if !validCurrency(row.Currency) {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[4], Code: "invalid_currency", Message: "币种必须为 3 位大写字母"})
	}
	billedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(record[5]))
	if err != nil {
		issues = append(issues, ValidationIssue{Line: line, Field: CSVHeaders[5], Code: "invalid_billed_at", Message: "账单时间必须是带时区的 RFC3339 时间"})
	} else {
		row.BilledAt = billedAt.UTC()
	}
	return row, issues
}

func hashRow(row CSVRow) string {
	canonical := strings.Join([]string{
		normalizeCarrier(row.Carrier), row.TrackingNo, row.ExternalLineID,
		strconv.FormatInt(row.AmountMinor, 10), row.Currency, row.BilledAt.UTC().Format(time.RFC3339Nano),
	}, "\x1f")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func normalizeCarrier(value string) string {
	return strings.Join(strings.Fields(strings.ToUpper(strings.TrimSpace(value))), " ")
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

func containsControl(value string) bool {
	for _, char := range value {
		if unicode.IsControl(char) {
			return true
		}
	}
	return false
}
