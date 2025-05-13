package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var APIKEY string

func getAllSymbols(currentDateStr string) ([]string, error) {
	// ใช้ URL ของ API
	url := "https://www.setsmart.com/api/listed-company-api/eod-price-by-security-type"

	// ตั้งค่า request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("api-key", APIKEY)

	// เพิ่ม query parameters
	q := req.URL.Query()
	q.Add("securityType", "CS")
	q.Add("date", "2025-05-09") // วันที่มีข้อมูล
	q.Add("adjustedPriceFlag", "Y")
	req.URL.RawQuery = q.Encode()

	// ส่งคำขอ
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// อ่านข้อมูลที่ได้รับ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// พิมพ์ตัวอย่างข้อมูลเพื่อดูโครงสร้าง
	fmt.Printf("ตัวอย่างข้อมูล (50 ตัวแรก): %s\n", string(body[:min(50, len(body))]))

	// ทำตามขั้นตอนพิเศษเพื่อตรวจสอบว่าเป็น object หรือ array
	trimmedBody := strings.TrimSpace(string(body))
	isArray := strings.HasPrefix(trimmedBody, "[")

	var symbols []string

	if isArray {
		// กรณีที่เป็น array
		var data []map[string]interface{}
		err = json.Unmarshal(body, &data)
		if err != nil {
			return nil, fmt.Errorf("แปลงข้อมูล JSON Array ไม่สำเร็จ: %v", err)
		}

		symbols = make([]string, 0, len(data))
		for _, item := range data {
			if symbol, ok := item["symbol"].(string); ok {
				symbols = append(symbols, symbol)
			}
		}
	} else {
		// กรณีที่เป็น object
		var objData map[string]interface{}
		err = json.Unmarshal(body, &objData)
		if err != nil {
			return nil, fmt.Errorf("แปลงข้อมูล JSON Object ไม่สำเร็จ: %v", err)
		}

		// พิมพ์โครงสร้าง keys ของ object
		fmt.Println("โครงสร้างข้อมูล (keys):")
		for key, value := range objData {
			fmt.Printf("- %s (type: %T)\n", key, value)
		}

		// ตรวจสอบหลายเส้นทางที่อาจจะเป็นไปได้ว่า object มี array ของข้อมูลหุ้นอยู่ตรงไหน
		possiblePaths := []string{"data", "items", "list", "stocks", "symbols", "result", "results", "securities"}

		foundData := false
		for _, path := range possiblePaths {
			if dataArray, ok := objData[path].([]interface{}); ok {
				fmt.Printf("พบข้อมูลใน field '%s' จำนวน %d รายการ\n", path, len(dataArray))

				symbols = make([]string, 0, len(dataArray))
				for _, item := range dataArray {
					if stockObj, ok := item.(map[string]interface{}); ok {
						if symbol, ok := stockObj["symbol"].(string); ok {
							symbols = append(symbols, symbol)
						}
					}
				}

				foundData = true
				break
			}
		}

		if !foundData {
			// กรณีพิเศษ: อาจเป็น object ที่มี key เป็นชื่อหุ้น
			symbols = make([]string, 0, len(objData))
			for key := range objData {
				// ตรวจสอบว่า key อาจจะเป็นสัญลักษณ์หุ้นหรือไม่
				// (ปกติสัญลักษณ์หุ้นจะเป็นตัวอักษรไม่เกิน 20 ตัว)
				if len(key) <= 20 && !strings.Contains(key, " ") {
					symbols = append(symbols, key)
				}
			}
		}
	}

	// แสดงผลลัพธ์
	fmt.Printf("จำนวนสัญลักษณ์หุ้นที่พบ: %d\n", len(symbols))
	if len(symbols) > 0 {
		showCount := min(5, len(symbols))
		fmt.Printf("ตัวอย่างสัญลักษณ์: %v\n", symbols[:showCount])
	} else {
		// ถ้าไม่พบข้อมูลเลย ให้พิมพ์ข้อมูลทั้งหมดเพื่อตรวจสอบ
		fmt.Println("ไม่พบข้อมูลสัญลักษณ์หุ้น ข้อมูลที่ได้รับ:")
		fmt.Println(string(body))
	}

	return symbols, nil
}

func main() {

	godotenv.Load()
	// ใส่ connection string ที่คุณได้รับจาก MongoDB Atlas
	// ตัวอย่าง: mongodb+srv://username:password@cluster0.mongodb.net/mydb?retryWrites=true&w=majority
	//uri := os.Getenv("MONGO_URL")

	APIKEY = os.Getenv("API_KEY")

	//// สร้าง client options
	//clientOptions := options.Client().ApplyURI(uri)
	//
	//// กำหนด context ที่มี timeout
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	//
	//// เชื่อมต่อกับ MongoDB
	//client, err := mongo.Connect(clientOptions)
	//if err != nil {
	//	log.Fatal("ไม่สามารถเชื่อมต่อกับ MongoDB ได้:", err)
	//}
	//
	//// ตรวจสอบการเชื่อมต่อ
	//err = client.Ping(ctx, nil)
	//if err != nil {
	//	log.Fatal("ไม่สามารถ ping ไปยัง MongoDB ได้:", err)
	//}
	//
	//fmt.Println("เชื่อมต่อกับ MongoDB Atlas สำเร็จแล้ว!")

	financialData, err := getAllFinancialDataCombined()
	if err != nil {
		fmt.Printf("เกิดข้อผิดพลาดในการดึงข้อมูล: %v\n", err)
		return
	}

	// ส่งออกเป็นไฟล์ CSV
	outputFile := "stock_financial_data.csv"
	if err := ExportToCSV(financialData, outputFile); err != nil {
		fmt.Printf("เกิดข้อผิดพลาดในการส่งออกไฟล์ CSV: %v\n", err)
		return
	}
}
