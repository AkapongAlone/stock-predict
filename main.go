package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"log"
	"net/http"
	"time"
)

var APIKEY string

func main() {

	godotenv.Load()
	// ใส่ connection string ที่คุณได้รับจาก MongoDB Atlas
	// ตัวอย่าง: mongodb+srv://username:password@cluster0.mongodb.net/mydb?retryWrites=true&w=majority
	uri := "mongodb+srv://your-username:your-password@your-cluster-url/test?retryWrites=true&w=majority"

	// สร้าง client options
	clientOptions := options.Client().ApplyURI(uri)

	// กำหนด context ที่มี timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// เชื่อมต่อกับ MongoDB
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		log.Fatal("ไม่สามารถเชื่อมต่อกับ MongoDB ได้:", err)
	}

	// ตรวจสอบการเชื่อมต่อ
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("ไม่สามารถ ping ไปยัง MongoDB ได้:", err)
	}

	fmt.Println("เชื่อมต่อกับ MongoDB Atlas สำเร็จแล้ว!")

}

func getListCompanyName() ([]ListedCompanyEODPriceBySecurityType, error) {
	var result []ListedCompanyEODPriceBySecurityType

	req, err := http.NewRequest("GET", "https://api.example.com/products", nil)
	if err != nil {
		fmt.Println("เกิดข้อผิดพลาด:", err)
		return nil, err
	}

	req.Header.Add("api-key", APIKEY)

	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("เกิดข้อผิดพลาดในการส่ง request:", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API ส่งค่า error กลับมา: %d\n", resp.StatusCode)
		return nil, err
	}

	// แปลงข้อมูล JSON เป็น struct
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		fmt.Println("เกิดข้อผิดพลาดในการแปลงข้อมูล:", err)
		return nil, err
	}
}
