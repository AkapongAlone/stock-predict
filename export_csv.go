package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

// สำหรับการแปลงชื่อคอลัมน์เป็นภาษาไทย (ถ้าต้องการ)
var columnThaiNames = map[string]string{
	"Symbol":             "หุ้น",
	"Year":               "ปี",
	"Quarter":            "ไตรมาส",
	"TotalAssets":        "สินทรัพย์รวม",
	"TotalLiabilities":   "หนี้สินรวม",
	"ShareholderEquity":  "ส่วนของผู้ถือหุ้น",
	"TotalRevenue":       "รายได้รวม",
	"NetProfit":          "กำไรสุทธิ",
	"EPS":                "กำไรต่อหุ้น",
	"ROE":                "อัตราผลตอบแทนส่วนของผู้ถือหุ้น",
	"ROA":                "อัตราผลตอบแทนจากสินทรัพย์",
	"DE":                 "อัตราส่วนหนี้สินต่อส่วนของผู้ถือหุ้น",
	"PriceClose":         "ราคาปิด",
	"PricePE":            "P/E",
	"PricePBV":           "P/BV",
	"PriceDividendYield": "อัตราเงินปันผลตอบแทน",
	"PriceMarketCap":     "มูลค่าตลาด",
}

// ExportToCSV - ส่งออกข้อมูลงบการเงินเป็นไฟล์ CSV
func ExportToCSV(data []FinancialData, filename string) error {
	if len(data) == 0 {
		return fmt.Errorf("ไม่มีข้อมูลสำหรับส่งออก")
	}

	// กำหนดชื่อไฟล์ถ้าไม่ได้ระบุ
	if filename == "" {
		timestamp := time.Now().Format("20060102_150405")
		filename = fmt.Sprintf("financial_data_%s.csv", timestamp)
	}

	// สร้างไฟล์ CSV
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("ไม่สามารถสร้างไฟล์ CSV: %v", err)
	}
	defer file.Close()

	_, err = file.Write([]byte{0xEF, 0xBB, 0xBF})
	if err != nil {
		return err
	}

	// สร้าง writer สำหรับเขียนไฟล์ CSV
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// กำหนดคอลัมน์ที่ต้องการส่งออก
	// ส่วนของข้อมูลพื้นฐานจาก FinancialData
	baseColumns := []string{
		"Symbol", "Year", "Quarter", "DateAsof", "TotalAssets", "TotalLiabilities",
		"PaidupShareCapital", "ShareholderEquity", "TotalEquity",
		"TotalRevenueQuarter", "TotalRevenueAccum", "TotalExpensesQuarter", "TotalExpensesAccum",
		"EbitQuarter", "EbitAccum", "NetProfitQuarter", "NetProfitAccum",
		"EpsQuarter", "EpsAccum", "OperatingCashFlow", "InvestingCashFlow", "FinancingCashFlow",
		"ROE", "ROA", "NetProfitMarginQuarter", "NetProfitMarginAccum", "DE",
		"FixedAssetTurnover", "TotalAssetTurnover",
	}

	// ส่วนของข้อมูลราคาจาก PriceData (ถ้ามี)
	priceColumns := []string{
		"price_close", "price_pe", "price_pbv", "price_dividendYield", "price_marketCap",
		"price_totalVolume", "price_high", "price_low", "price_open", "price_prior",
	}

	// รวมคอลัมน์ทั้งหมด
	allColumns := append(baseColumns, priceColumns...)

	// แปลงเป็นภาษาไทย (ถ้าต้องการ)
	thaiColumnNames := make([]string, len(allColumns))
	for i, col := range allColumns {
		if thaiName, ok := columnThaiNames[col]; ok {
			thaiColumnNames[i] = thaiName
		} else if thaiName, ok := columnThaiNames[col[6:]]; ok && len(col) > 6 && col[:6] == "price_" {
			// สำหรับคอลัมน์ราคาที่ขึ้นต้นด้วย price_
			thaiColumnNames[i] = thaiName
		} else {
			thaiColumnNames[i] = col
		}
	}

	// เขียนหัวคอลัมน์
	if err := writer.Write(thaiColumnNames); err != nil {
		return fmt.Errorf("ไม่สามารถเขียนหัวคอลัมน์: %v", err)
	}

	// เขียนข้อมูลแต่ละแถว
	for _, item := range data {
		// สร้างแถวข้อมูลว่าง
		row := make([]string, len(allColumns))

		// เติมข้อมูลพื้นฐาน
		for i, col := range baseColumns {
			switch col {
			case "Symbol":
				row[i] = item.Symbol
			case "Year":
				row[i] = item.Year
			case "Quarter":
				row[i] = item.Quarter
			case "DateAsof":
				row[i] = item.DateAsof
			case "TotalAssets":
				row[i] = formatFloat(item.TotalAssets)
			case "TotalLiabilities":
				row[i] = formatFloat(item.TotalLiabilities)
			case "PaidupShareCapital":
				row[i] = formatFloat(item.PaidupShareCapital)
			case "ShareholderEquity":
				row[i] = formatFloat(item.ShareholderEquity)
			case "TotalEquity":
				row[i] = formatFloat(item.TotalEquity)
			case "TotalRevenueQuarter":
				row[i] = formatFloat(item.TotalRevenueQuarter)
			case "TotalRevenueAccum":
				row[i] = formatFloat(item.TotalRevenueAccum)
			case "TotalExpensesQuarter":
				row[i] = formatFloat(item.TotalExpensesQuarter)
			case "TotalExpensesAccum":
				row[i] = formatFloat(item.TotalExpensesAccum)
			case "EbitQuarter":
				row[i] = formatFloat(item.EbitQuarter)
			case "EbitAccum":
				row[i] = formatFloat(item.EbitAccum)
			case "NetProfitQuarter":
				row[i] = formatFloat(item.NetProfitQuarter)
			case "NetProfitAccum":
				row[i] = formatFloat(item.NetProfitAccum)
			case "EpsQuarter":
				row[i] = formatFloat(item.EpsQuarter)
			case "EpsAccum":
				row[i] = formatFloat(item.EpsAccum)
			case "OperatingCashFlow":
				row[i] = formatFloat(item.OperatingCashFlow)
			case "InvestingCashFlow":
				row[i] = formatFloat(item.InvestingCashFlow)
			case "FinancingCashFlow":
				row[i] = formatFloat(item.FinancingCashFlow)
			case "ROE":
				row[i] = formatFloat(item.Roe)
			case "ROA":
				row[i] = formatFloat(item.Roa)
			case "NetProfitMarginQuarter":
				row[i] = formatFloat(item.NetProfitMarginQuarter)
			case "NetProfitMarginAccum":
				row[i] = formatFloat(item.NetProfitMarginAccum)
			case "DE":
				row[i] = formatFloat(item.De)
			case "FixedAssetTurnover":
				row[i] = formatFloat(item.FixedAssetTurnover)
			case "TotalAssetTurnover":
				row[i] = formatFloat(item.TotalAssetTurnover)
			}
		}

		// เติมข้อมูลราคา (ถ้ามี)
		if item.PriceData != nil {
			for i, col := range priceColumns {
				idx := i + len(baseColumns)
				if val, ok := item.PriceData[col]; ok {
					// แปลงค่าตามประเภทข้อมูล
					switch v := val.(type) {
					case float64:
						row[idx] = formatFloat(v)
					case string:
						row[idx] = v
					default:
						row[idx] = fmt.Sprintf("%v", v)
					}
				}
			}
		}

		// เขียนแถวข้อมูล
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("ไม่สามารถเขียนข้อมูลแถว: %v", err)
		}
	}

	fmt.Printf("ส่งออกข้อมูลเรียบร้อยแล้วที่ %s จำนวน %d รายการ\n", filename, len(data))
	return nil
}

// formatFloat - ฟังก์ชันช่วยแปลงตัวเลขทศนิยมเป็นสตริง
func formatFloat(f float64) string {
	if f == 0 {
		return "0"
	}

	// สำหรับตัวเลขที่มีค่ามาก ให้แสดงแบบไม่มีทศนิยม
	if f >= 1000000 || f <= -1000000 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	} else if f >= 1000 || f <= -1000 {
		return strconv.FormatFloat(f, 'f', 2, 64)
	}

	// สำหรับตัวเลขทั่วไป แสดง 4 ตำแหน่งทศนิยม
	return strconv.FormatFloat(f, 'f', 4, 64)
}
