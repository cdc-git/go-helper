package help

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type Jamet struct {
	Config map[string]*gorm.DB
	Redis  FormatRedis
	Log    string
}

type FormatRedis struct {
	Host, Port, Password string
	Database             int
	On                   bool
}

func NewJamet(param Jamet) *Jamet {
	return &Jamet{
		Config: param.Config,
		Redis:  param.Redis,
		Log:    param.Log,
	}
}

func (met *Jamet) GetData(table string, connection string) *gorm.DB {

	return met.Config[connection].Table(table)
}

func (met *Jamet) Connection(conn string) *gorm.DB {
	db := met.Config[conn]

	return db.Begin()
}

func (met *Jamet) SinchronizeID(db *gorm.DB, id string, char string, format int32) string {

	defer met.ErrorLog()

	var getMstRunNum map[string]interface{}
	db.Table("mst_run_nums").Where(map[string]interface{}{"val_id": id, "val_char": char}).Find(&getMstRunNum)

	var value string
	if len(getMstRunNum) != 0 {

		num, err := strconv.Atoi(getMstRunNum["val_value"].(string))
		if err != nil {
			panic(err)
		}

		num = num + 1
		db.Table("mst_run_nums").Where(map[string]interface{}{"val_id": id, "val_char": char}).Updates(map[string]interface{}{"val_value": num})

		value = fmt.Sprintf("%0*d", format, num)
	} else {
		value = fmt.Sprintf("%0*d", format, 1)
		InsertData(db, "mst_run_nums", map[string]interface{}{
			"id":        UUID(),
			"val_value": 1,
			"val_id":    id,
			"val_char":  char,
		})
	}

	return fmt.Sprintf("%s%s%s", id, char, value)
}


func (met *Jamet) CreateData(c *gin.Context, table *gorm.DB, field []string) map[string]interface{} {

	query := table
	for _, value := range field {
		check := c.Query(value)
		if check != "" {
			query.Where(value+" = ?", c.Query(value))
		}
	}

	var results []map[string]interface{}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		query.Find(&results)
	} else {
		query.Limit(limit).Find(&results)
	}

	return map[string]interface{}{
		"status": true,
		"data":   results,
	}
}

func (met *Jamet) CreateDataTable(c *gin.Context, table *gorm.DB, search []string) map[string]interface{} {

	defer met.ErrorLog()

	//MANDATORY
	draw, err := strconv.Atoi(c.PostForm("draw"))
	if err != nil {
		panic("draw not found")
	}

	limit, err := strconv.Atoi(c.PostForm("length"))
	if err != nil {
		panic("limit not found")
	}

	offset, err := strconv.Atoi(c.PostForm("start"))
	if err != nil {
		panic("offset not found")
	}

	query := table

	//WHERE
	inField := c.PostForm("in_field")
	inSearch := c.PostForm("in_search")

	if inSearch != "" {
		where := fmt.Sprintf("%s IN ?", inField)

		query.Where(where, [...]string{inSearch})
	}

	for i, val := range search {
		operator := fmt.Sprintf("tempOperator[%s]", val)
		field := fmt.Sprintf("tempSearch[%s]", val)

		value := c.PostForm(field)
		if value != "" {
			i++
			op := c.PostForm(operator)
			where := fmt.Sprintf("%s %s ?", val, op)

			query.Where(where, value)
		}

		//SEARCH VALUE
		searchBox := c.PostForm("search[value]")
		if searchBox != "" {
			if i == 0 {
				where := fmt.Sprintf("%s LIKE ?", val)
				query.Where(where, "%"+searchBox+"%")
			} else {
				where := fmt.Sprintf("%s LIKE ?", val)
				query.Or(where, "%"+searchBox+"%")
			}
		}
	}

	tempSort := c.PostForm("tempSort")

	if tempSort != "" {
		query.Order(tempSort)
	}

	var recordsTotal int64
	var results []map[string]interface{}

	query.Limit(limit).Offset(offset).Find(&results).Count(&recordsTotal)
	return map[string]interface{}{
		"status":          true,
		"draw":            draw,
		"data":            results,
		"recordsFiltered": recordsTotal,
		"recordsTotal":    recordsTotal,
	}
}

func (met *Jamet) GetRequest(c *gin.Context) []byte {

	defer met.ErrorLog()

	param := c.Request.URL.Query()

	mars, err := json.Marshal(param)
	if err != nil {
		panic(err)
	}

	buf, _ := io.ReadAll(c.Request.Body)
	body := io.NopCloser(bytes.NewBuffer(buf))

	c.Request.Body = body

	return append(mars, buf...)
}

func InsertData(db *gorm.DB, table string, data any) any {

	result := db.Table(table).Create(data).Error
	if result != nil {
		if mysqlErr, ok := result.(*mysql.MySQLError); ok {

			return mysqlErr.Message
		} else {
			return "Error saat menyimpan data!"
		}
	}

	return nil
}

// CACHE
func (met *Jamet) ReadCache(previx string) (bool, map[string]interface{}) {

	ctx := context.Background()

	format := met.Redis

	if format.On {
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", format.Host, format.Port),
			Password: format.Password, // No password set
			DB:       format.Database, // Use default DB
		})

		val, err := client.Get(ctx, previx).Result()
		if err != nil {
			met.LogError(fmt.Sprintf("Gagal dalam menulis cache %s", previx))
			return false, map[string]interface{}{}
		}

		convert := Converter(val)

		return true, convert

	} else {
		return false, map[string]interface{}{}
	}
}

func (met *Jamet) WriteCache(previx string, data any) {

	defer met.ErrorLog()

	ctx := context.Background()

	format := met.Redis

	if format.On {
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", format.Host, format.Port),
			Password: format.Password, // No password set
			DB:       format.Database, // Use default DB
		})

		jsonStr, err := json.Marshal(data)
		if err != nil {
			panic(err)
		}

		res, err := client.Set(ctx, previx, jsonStr, 0).Result()
		if err != nil {
			panic(err)
		}

		log.Println(res)
	}
}

// recover log after panic
func (met *Jamet) ErrorLog() {
	message := recover()

	if message != nil {
		log.Println(message)
		met.LogFatal(message.(string))
	} else {
		fmt.Println("---- No Error have a nice day  ----")
	}
}

/**
new update v0.17 --met.Logging

Debug 🛠 → Buat ngintip daleman kode, kayak investigasi detektif. Biasanya cuma buat developer pas lagi ngoding.
Info ℹ → Buat kasih tau sesuatu yang biasa aja, kayak "Aplikasi nyala nih!" atau "User login sukses".
Error ❌ → Ada masalah, tapi masih bisa jalan. Contoh: "Gagal simpen data, coba lagi ya!".
Fatal ☠ → Masalah gede banget sampe sistemnya KO. Contohnya: "Database ilang! Sistem mati total!".
Success ✅ → Buat ngumumin sesuatu berhasil, kayak "Orderan lo sukses, siap dikirim!".

"debug"
"info"
"error"
"fatal"
"success"
*/

func (met *Jamet) LogDebug(message string) {

	data, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Println("Error reading go.mod:", err)
		return
	}

	lines := strings.Split(string(data), "\n")

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":    "debug",
		"message": message,
		"module":  strings.Fields(lines[0])[1],
	})

	if err != nil {
		log.Fatalln(err)
	}

	met.Logging(jsonData)
}

func (met *Jamet) LogInfo(message string) {

	data, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Println("Error reading go.mod:", err)
		return
	}

	lines := strings.Split(string(data), "\n")

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":    "info",
		"message": message,
		"module":  strings.Fields(lines[0])[1],
	})

	if err != nil {
		log.Fatalln(err)
	}

	met.Logging(jsonData)
}

func (met *Jamet) LogError(message string) {

	data, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Println("Error reading go.mod:", err)
		return
	}

	lines := strings.Split(string(data), "\n")

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":    "error",
		"message": message,
		"module":  strings.Fields(lines[0])[1],
	})

	if err != nil {
		log.Fatalln(err)
	}

	met.Logging(jsonData)
}

func (met *Jamet) LogFatal(message string) {

	data, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Println("Error reading go.mod:", err)
		return
	}

	lines := strings.Split(string(data), "\n")

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":    "fatal",
		"message": message,
		"module":  strings.Fields(lines[0])[1],
	})

	if err != nil {
		log.Fatalln(err)
	}

	met.Logging(jsonData)
}

func (met *Jamet) LogSuccess(message string) {

	data, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Println("Error reading go.mod:", err)
		return
	}

	lines := strings.Split(string(data), "\n")

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":    "success",
		"message": message,
		"module":  strings.Fields(lines[0])[1],
	})

	if err != nil {
		log.Fatalln(err)
	}

	met.Logging(jsonData)
}

func (met *Jamet) Logging(body []byte) {

	defer met.ErrorLog()
	url := met.Log;

	if url != "" {
		// Create a new HTTP POST request.
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if err != nil {
			message := fmt.Sprintf("Error creating request: %s", err)
			panic(message)
		}
	
		req.Header.Set("Content-Type", "application/json")
	
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			message := fmt.Sprintf("Error sending request: %s", err)
			panic(message)
		}
	
		defer resp.Body.Close()
	
		log.Println("Response Status:", resp.Status)
	}
	
}

// end update logging
