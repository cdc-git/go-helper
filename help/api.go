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
	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Jamet struct {
	Config map[string]*gorm.DB
	Redis  map[string]FormatRedis
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

func (met *Jamet) CreateData(c *gin.Context, table *gorm.DB, field []string) map[string]interface{} {

	query := table
	for _, value := range field {
		check := c.Query(value)
		if check != "" {
			query.Where(value+" = ?", c.Query(value))
		}

		in_field := c.Query("in_field")
		in_search := c.Query("in_search")

		if in_field == value {

			query.Where(value+" in (?)", strings.Split(in_search, ","))
		}
	}

	var results []map[string]interface{}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		query.Limit(10).Find(&results)
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
		log.Println("draw not found")
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

	// //WHERE
	inField := c.PostForm("in_field")
	inSearch := c.PostForm("in_search")

	if inSearch != "" {
		where := fmt.Sprintf("%s IN ?", inField)

		query.Where(where, [...]string{inSearch})
	}

	var searchField string
	for i, field := range search {
		operator := fmt.Sprintf("tempOperator[%s]", field)
		find := fmt.Sprintf("tempSearch[%s]", field)

		value := c.PostForm(find)
		if value != "" {
			op := c.PostForm(operator)

			// Handle different operators
			switch op {
			case "LIKE", "NOT LIKE":
				query.Where(fmt.Sprintf("%s %s ?", field, op), "%"+value+"%")
			case "IN", "NOT IN":
				query.Where(fmt.Sprintf("%s %s (?)", field, op), strings.Split(value, ","))
			case "IS", "IS NOT":
				if strings.ToUpper(value) == "NULL" {
					query.Where(fmt.Sprintf("%s %s NULL", field, op))
				} else {
					query.Where(fmt.Sprintf("%s %s ?", field, op), value)
				}
			default:
				query.Where(fmt.Sprintf("%s %s ?", field, op), value)
			}

		}

		//SEARCH VALUE
		searchBox := c.PostForm("search[value]")
		if searchBox != "" {

			searchField += field + " LIKE '%" + searchBox + "%'"
			if i != len(search)-1 {
				searchField += " OR "
			}
		}
	}

	if searchField != "" {
		query.Where(fmt.Sprintf("(%s)", searchField))
	}

	tempSort := c.PostForm("tempSort")

	if tempSort != "" {
		query.Order(tempSort)
	}

	//unmarshal request
	var req map[string]interface{}
	if err := json.Unmarshal([]byte(c.PostForm("alfred_hari_bersatu")), &req); err != nil {
		fmt.Println(err)
	}

	if req["order"] != nil {
		ordering := req["order"].([]interface{})

		for i := 0; i < len(ordering); i++ {
			columnIndex := c.PostForm(fmt.Sprintf("order[%d][column]", i))
			dir := c.PostForm(fmt.Sprintf("order[%d][dir]", i))
	
			column := c.PostForm(fmt.Sprintf("columns[%v][data]", columnIndex))
	
			query.Order(column + " " + dir)
		}
	}

	var recordsTotal int64
	var results []map[string]interface{}

	query.Count(&recordsTotal)
	query.Limit(limit).Offset(offset).Find(&results)

	return map[string]interface{}{
		"status":          true,
		"draw":            draw,
		"data":            results,
		"recordsFiltered": recordsTotal,
		"recordsTotal":    recordsTotal,
	}
}

// TRANSACTION
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
func (met *Jamet) ReadCache(previx string, d string) (bool, map[string]interface{}) {

	ctx := context.Background()

	format := met.Redis[d]

	if format.On {
		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", format.Host, format.Port),
			Password: format.Password, // No password set
			DB:       format.Database, // Use default DB
		})

		val, err := client.Get(ctx, previx).Result()
		if err != nil {
			met.LogError(err.Error())
			met.LogError(fmt.Sprintf("Gagal dalam membaca cache %s", previx))
			return false, map[string]interface{}{}
		}

		convert := Converter(val)

		return true, convert

	} else {
		return false, map[string]interface{}{}
	}
}

func (met *Jamet) WriteCache(previx string, data any, d string) {

	defer met.ErrorLog()

	ctx := context.Background()

	format := met.Redis[d]

	if format.On && data != nil {

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

func (met *Jamet) DelCache(previx string, d string) {
	defer met.ErrorLog()

	format := met.Redis[d]
	if format.On {

		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", format.Host, format.Port),
			Password: format.Password, // No password set
			DB:       format.Database, // Use default DB
		})

		ctx := context.Background()

		var keys []string
		var err error
		var cursor uint64

		keys, cursor, err = client.Scan(ctx, cursor, fmt.Sprintf("*%s*", previx), 10).Result()
		if err != nil {
			panic(err)
		}

		fmt.Println("number of change", cursor)

		var n int
		for _, p := range keys {
			_, err := client.Del(ctx, p).Result()
			if err != nil {
				panic(err)
			}

			n++
		}

		fmt.Printf("deleted %d keys\n", n)
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

func (met *Jamet) LogCustom(tipe string, message string) {

	data, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Println("Error reading go.mod:", err)
		return
	}

	lines := strings.Split(string(data), "\n")

	jsonData, err := json.Marshal(map[string]interface{}{
		"type":    tipe,
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
	url := met.Log

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

// update request
func (met *Jamet) PostXT(url string, body []byte, header map[string]string) map[string]interface{} {

	defer met.ErrorLog()

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		message := fmt.Sprintf("Error creating request: %s", err)
		panic(message)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", string(len(body)))

	if len(header) > 0 {

		for key, val := range header {
			req.Header.Set(key, val)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		message := fmt.Sprintf("Error sending request: %s", err)
		panic(message)
	}

	defer resp.Body.Close()

	log.Println("Response Status:", resp.Status)

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Sprintf("Error reading response body: %s", err))
	}

	var data map[string]interface{}
	err = json.Unmarshal(response, &data)
	if err != nil {
		msg := fmt.Sprint("Error unmarshaling JSON:", err)
		panic(msg)
	}

	return data
}
