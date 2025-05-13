package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/sourcegraph/conc/pool"
	"golang.org/x/time/rate"
)

// FinancialData และ PriceData structs ยังคงเหมือนเดิม

// สร้าง global rate limiter เพื่อป้องกันการส่ง request มากเกินไป
var limiter = rate.NewLimiter(rate.Every(50*time.Millisecond), 10) // 20 req/sec

func getAllFinancialDataCombined() ([]FinancialData, error) {
	// คำนวณวันที่ปัจจุบันและวันที่ย้อนหลัง 5 ปี
	now := time.Now()
	fiveYearsAgo := now.AddDate(-5, 0, 0)

	currentYear := now.Year()
	currentMonth := int(now.Month())
	currentQuarter := ((currentMonth - 1) / 3) + 1

	startYear := fiveYearsAgo.Year()
	startMonth := int(fiveYearsAgo.Month())
	startQuarter := ((startMonth - 1) / 3) + 1

	currentDateStr := now.Format("2006-01-02")

	// 1. ดึงรายชื่อหุ้นทั้งหมด
	symbols, err := getAllSymbols(currentDateStr)
	if err != nil {
		return nil, fmt.Errorf("ไม่สามารถดึงรายชื่อหุ้นได้: %v", err)
	}

	fmt.Printf("พบหุ้นทั้งหมด %d ตัว\n", len(symbols))

	// 2. สร้าง channel สำหรับรับข้อมูลจาก goroutines
	resultsChan := make(chan []FinancialData, len(symbols))

	// 3. สร้าง context พร้อม timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30*len(symbols))*time.Second)
	defer cancel()

	// 4. ใช้ conc pool สำหรับการทำงานแบบขนาน
	p := pool.New().WithContext(ctx).WithMaxGoroutines(20) // ปรับจำนวน goroutines ให้เหมาะสม

	// 5. สร้าง ErrorCollector สำหรับเก็บข้อผิดพลาด
	var errorCollector sync.Map

	// 6. วนลูปดึงข้อมูลแต่ละบริษัท
	for i, symbol := range symbols {
		symbol := symbol // ป้องกัน closure capturing loop variable
		idx := i

		p.Go(func(ctx context.Context) error {
			fmt.Printf("กำลังดึงข้อมูลของ %s (%d/%d)\n", symbol, idx+1, len(symbols))

			// สร้าง HTTP client ที่สามารถยกเลิกได้ด้วย context
			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			// ดึงข้อมูลงบการเงิน
			financialData, err := fetchFinancialData(ctx, client, symbol, startYear, startQuarter, currentYear, currentQuarter)
			if err != nil {
				errorCollector.Store(symbol, fmt.Sprintf("ข้อมูลงบการเงิน: %v", err))
				return nil // ไม่ต้องการให้หยุดทั้งหมดเมื่อบริษัทเดียวล้มเหลว
			}

			// ดึงข้อมูลราคาสำหรับแต่ละไตรมาส
			if len(financialData) > 0 {
				err = fetchPriceData(ctx, client, financialData)
				if err != nil {
					// บันทึกข้อผิดพลาดแต่ยังคงส่งข้อมูลงบการเงินที่มีอยู่
					errorCollector.Store(symbol+"-price", fmt.Sprintf("ข้อมูลราคา: %v", err))
				}
			}

			// ส่งข้อมูลกลับเข้า channel (เฉพาะเมื่อมีข้อมูล)
			if len(financialData) > 0 {
				select {
				case resultsChan <- financialData:
					// ส่งข้อมูลเรียบร้อย
				case <-ctx.Done():
					// ถูกยกเลิกหรือ timeout
					return ctx.Err()
				}
			}

			return nil
		})

		fmt.Printf("เสร็จงาน %s (%d/%d)\n", symbol, idx+1, len(symbols))
	}

	// 7. รอให้งานทั้งหมดเสร็จสิ้น
	if err := p.Wait(); err != nil {
		fmt.Printf("เกิดข้อผิดพลาดในการดึงข้อมูล: %v\n", err)
		// ทำต่อแม้จะมี error บางส่วน
	}

	// 8. ปิด channel resultsChan
	close(resultsChan)

	// 9. รวบรวมข้อมูลทั้งหมดจาก channel
	var combinedData []FinancialData
	for data := range resultsChan {
		combinedData = append(combinedData, data...)
	}

	// 10. บันทึกข้อผิดพลาดที่เกิดขึ้นระหว่างการทำงาน
	errorsLogFile := "fetch_errors.log"
	var errorsFound int
	errorLog, err := os.Create(errorsLogFile)
	if err == nil {
		defer errorLog.Close()
		fmt.Fprintf(errorLog, "--- ข้อผิดพลาดในการดึงข้อมูลวันที่ %s ---\n", time.Now().Format("2006-01-02 15:04:05"))
		errorCollector.Range(func(key, value interface{}) bool {
			fmt.Fprintf(errorLog, "%v: %v\n", key, value)
			errorsFound++
			return true
		})
		if errorsFound > 0 {
			fmt.Printf("พบข้อผิดพลาด %d รายการ บันทึกไว้ที่ %s\n", errorsFound, errorsLogFile)
		}
	}

	// 11. เรียงลำดับข้อมูลตามชื่อหุ้น ปี และไตรมาส (ล่าสุดก่อน)
	if len(combinedData) > 0 {
		sort.Slice(combinedData, func(i, j int) bool {
			// เรียงตามชื่อหุ้น (A-Z)
			if combinedData[i].Symbol != combinedData[j].Symbol {
				return combinedData[i].Symbol < combinedData[j].Symbol
			}

			// แปลงปีเป็นตัวเลข
			yearI, _ := strconv.Atoi(combinedData[i].Year)
			yearJ, _ := strconv.Atoi(combinedData[j].Year)

			// เรียงตามปี (ล่าสุดก่อน)
			if yearI != yearJ {
				return yearI > yearJ
			}

			// แปลงไตรมาสเป็นตัวเลข
			quarterI, _ := strconv.Atoi(combinedData[i].Quarter)
			quarterJ, _ := strconv.Atoi(combinedData[j].Quarter)

			// เรียงตามไตรมาส (ล่าสุดก่อน)
			return quarterI > quarterJ
		})
	}

	fmt.Printf("ดึงข้อมูลสำเร็จ: %d รายการ จาก %d บริษัท\n", len(combinedData), len(symbols))

	return combinedData, nil
}

// แยกการดึงข้อมูลงบการเงินเป็นฟังก์ชันแยก
func fetchFinancialData(ctx context.Context, client *http.Client, symbol string, startYear, startQuarter, endYear, endQuarter int) ([]FinancialData, error) {
	// จำกัดอัตราการเรียก API
	if err := limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %v", err)
	}

	url := "https://www.setsmart.com/api/listed-company-api/financial-data-and-ratio-by-symbol"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("สร้างคำขอไม่สำเร็จ: %v", err)
	}

	// เพิ่ม headers
	req.Header.Add("api-key", APIKEY)

	// เพิ่ม query parameters
	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("startYear", strconv.Itoa(startYear))
	q.Add("startQuarter", strconv.Itoa(startQuarter))
	q.Add("endYear", strconv.Itoa(endYear))
	q.Add("endQuarter", strconv.Itoa(endQuarter))
	req.URL.RawQuery = q.Encode()

	// ส่งคำขอ
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ส่งคำขอไม่สำเร็จ: %v", err)
	}
	defer resp.Body.Close()

	// อ่านข้อมูลที่ได้รับ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("อ่านข้อมูลไม่สำเร็จ: %v", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API ตอบสถานะ: %d", resp.StatusCode)
	}

	// แปลง JSON เป็นโครงสร้างข้อมูล
	var data []FinancialData
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("แปลงข้อมูล JSON ไม่สำเร็จ: %v", err)
	}

	return data, nil
}

// แยกการดึงข้อมูลราคาเป็นฟังก์ชันแยก
func fetchPriceData(ctx context.Context, client *http.Client, financialData []FinancialData) error {
	// สร้าง wait group เพื่อรอให้การดึงข้อมูลราคาทั้งหมดเสร็จสิ้น

	// สร้าง mutex เพื่อป้องกันการเขียนข้อมูลพร้อมกัน
	var mutex sync.Mutex

	// สร้าง sub-context เพื่อให้สามารถยกเลิกงานย่อยได้
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	wg := pool.New().WithErrors().WithContext(subCtx)
	// ดึงข้อมูลราคาสำหรับแต่ละรายการในงบการเงิน
	for idx := range financialData {
		idx := idx

		wg.Go(func(ctx context.Context) error {

			// ตรวจสอบว่า context ถูกยกเลิกหรือไม่
			select {
			case <-subCtx.Done():
				return nil
			default:
				// ดำเนินการต่อ
			}

			quarterYear := financialData[idx].Year
			quarter := financialData[idx].Quarter
			symbol := financialData[idx].Symbol

			// หาวันที่สิ้นสุดไตรมาส (ประมาณการ)
			var quarterEndMonth int
			switch quarter {
			case "1":
				quarterEndMonth = 3
			case "2":
				quarterEndMonth = 6
			case "3":
				quarterEndMonth = 9
			default: // quarter == "4"
				quarterEndMonth = 12
			}

			quarterEndDate := fmt.Sprintf("%s-%02d-28", quarterYear, quarterEndMonth)

			// จำกัดอัตราการเรียก API
			if err := limiter.Wait(subCtx); err != nil {
				fmt.Printf("%s Q%s/%s: rate limit error: %v", symbol, quarter, quarterYear, err)
				return err
			}

			// ดึงข้อมูลราคาใกล้เคียงวันสิ้นสุดไตรมาส
			priceUrl := "https://www.setsmart.com/api/listed-company-api/eod-price-by-symbol"

			priceReq, err := http.NewRequestWithContext(subCtx, "GET", priceUrl, nil)
			if err != nil {
				fmt.Printf("%s Q%s/%s: สร้างคำขอราคาไม่สำเร็จ: %v", symbol, quarter, quarterYear, err)
				return err
			}

			// เพิ่ม headers
			priceReq.Header.Add("api-key", APIKEY)

			// เพิ่ม query parameters
			pq := priceReq.URL.Query()
			pq.Add("symbol", symbol)
			pq.Add("startDate", quarterEndDate)
			pq.Add("adjustedPriceFlag", "Y")
			priceReq.URL.RawQuery = pq.Encode()

			// ส่งคำขอ
			priceResp, err := client.Do(priceReq)
			if err != nil {
				fmt.Printf("%s Q%s/%s: ส่งคำขอราคาไม่สำเร็จ: %v", symbol, quarter, quarterYear, err)
				return err
			}

			// อ่านข้อมูลที่ได้รับ
			priceBody, err := io.ReadAll(priceResp.Body)
			priceResp.Body.Close()
			if err != nil {
				fmt.Printf("%s Q%s/%s: อ่านข้อมูลราคาไม่สำเร็จ: %v", symbol, quarter, quarterYear, err)
				return err
			}

			if priceResp.StatusCode != 200 {
				fmt.Printf("%s Q%s/%s: API ราคาตอบสถานะ: %d", symbol, quarter, quarterYear, priceResp.StatusCode)
				return err
			}

			// แปลง JSON เป็นโครงสร้างข้อมูล
			var priceData []PriceData
			err = json.Unmarshal(priceBody, &priceData)
			if err != nil {
				fmt.Printf("%s Q%s/%s: แปลงข้อมูล JSON ของราคาไม่สำเร็จ: %v", symbol, quarter, quarterYear, err)
				return err
			}

			if len(priceData) > 0 {
				// ล็อคเพื่อป้องกันการเขียนข้อมูลพร้อมกัน
				mutex.Lock()

				// แปลงข้อมูลราคาเป็น map[string]interface{} เพื่อเก็บใน PriceData
				financialData[idx].PriceData = make(map[string]interface{})

				// สร้าง map จาก PriceData
				latestPrice := priceData[0]
				priceJSON, _ := json.Marshal(latestPrice)
				var priceMap map[string]interface{}
				json.Unmarshal(priceJSON, &priceMap)

				// เพิ่มข้อมูลราคาลงใน PriceData
				for key, value := range priceMap {
					financialData[idx].PriceData["price_"+key] = value
				}

				mutex.Unlock()
			}
			return nil
		})
	}

	// รอให้การดึงข้อมูลราคาทั้งหมดเสร็จสิ้น
	err := wg.Wait()

	if err != nil {
		return fmt.Errorf("พบข้อผิดพลาดในการดึงข้อมูลราคา %v", err)
	}

	return nil
}
