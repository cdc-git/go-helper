package help

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	ranmath "math/rand"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// get UUID
func UUID() string {
	return uuid.New().String()
}

// return JSON status true
func PrintJSON(c *gin.Context, response any) {
	c.Render(http.StatusOK, render.JSON{Data: response})
}

// return JSON status false
func EPrintJSON(c *gin.Context, response any) {
	c.Render(http.StatusBadRequest, render.JSON{Data: response})
}

func ArrayKey(ar map[string]interface{}) []string {

	keys := make([]string, 0, len(ar))
	for k := range ar {
		keys = append(keys, k)
	}

	return keys
}

func InArray(s []string, str string) (bool, string) {
	for i, v := range s {
		if strings.Contains(v, str) {
			index := fmt.Sprintf("%d", i)
			return true, index
		}
	}
	return false, ""
}

func Contains(tmp []string, str string) (bool, string) {
	for i, val := range tmp {

		if strings.Contains(str, val) {
			index := fmt.Sprintf("%d", i)
			return true, index
		}
	}

	return false, ""
}

func Validation(request map[string]interface{}, format map[string]map[string]string) string {

	var errMessage []string
	message := format["message"]
	alias := format["alias"]
	for key, value := range format["field"] {

		var contain bool
		var index string
		cond := strings.Split(value, "|")
		formData := reflect.ValueOf(request[key]).String()

		contain, _ = InArray(cond, "required")
		if contain && formData == "" {

			if len(message) > 0 && message[key] != "" {
				errMessage = append(errMessage, message[key])
			} else {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage, fmt.Sprintf("%s perlu diisi", alias[key]))
				} else {
					errMessage = append(errMessage, fmt.Sprintf("%s perlu diisi", key))
				}
			}
		}

		//min
		contain, index = InArray(cond, "min")
		if contain {

			i, err := strconv.Atoi(index)
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			arr := strings.Split(cond[i], ":")
			min, err := strconv.Atoi(arr[1])
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			if len(formData) < min && formData != "" {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s kurang dari %d", alias[key], min))
				} else {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s kurang dari %d", key, min))
				}
				continue
			}
		}

		//max
		contain, index = InArray(cond, "max")
		if contain {

			i, err := strconv.Atoi(index)
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			arr := strings.Split(cond[i], ":")
			max, err := strconv.Atoi(arr[1])
			if err != nil {
				errMessage = append(errMessage, "Error pada saat validasi")
				break
			}

			if len(formData) > max && formData != "" {
				if len(alias) > 0 && alias[key] != "" {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s lebih dari %d", alias[key], max))
				} else {
					errMessage = append(errMessage, fmt.Sprintf("Panjang %s lebih dari %d", key, max))
				}
				continue
			}
		}

	}

	if len(errMessage) > 0 {
		return strings.Join(errMessage, " | ")
	} else {
		return ""
	}
}

func Md5(data []byte) string {

	return fmt.Sprintf("%x", md5.Sum(data))
}

func Converter(req any) map[string]interface{} {

	convert := fmt.Sprintf(`[%s]`, req)
	var jsonBlob = []byte(convert)
	var objmap []map[string]interface{}
	if err := json.Unmarshal(jsonBlob, &objmap); err != nil {
		log.Fatal(err)
	}

	return objmap[0]
}

func Unrupiah(nilai string) int64 {
	rupiah := strings.ReplaceAll(nilai, ",", "")
	result, err := strconv.Atoi(strings.ReplaceAll(rupiah, ".", ""))
	if err != nil {
		log.Fatalln(err)
	}

	return int64(result)
}

type DateTime struct {
	Data   any
	Format string
}

func DateFormat(data DateTime) (string, int64) {

	if data.Format == "" {
		data.Format = "Y-m-d H:i:s"
	}

	s := strings.Split(data.Format, " ")

	var (
		date   string
		hour   string
		format string
	)

	date = s[0]

	ds := []string{"-", " ", "/", ""}

	//date
	status, i := Contains(ds, date)
	if status {

		index, err := strconv.Atoi(i)
		if err != nil {
			panic("error on formatting date time")
		}

		d := strings.Split(date, string(ds[index]))

		var tmp []string

		for i := 0; i < len(d); i++ {
			switch d[i] {
			case "Y":
				tmp = append(tmp, "2006")
			case "y":
				tmp = append(tmp, "06")
			case "M":
				tmp = append(tmp, "Jan")
			case "m":
				tmp = append(tmp, "01")
			case "F":
				tmp = append(tmp, "January")
			case "d":
				tmp = append(tmp, "02")
			case "D":
				tmp = append(tmp, "Mon")
			}
		}

		format += strings.Join(tmp, ds[index])
	}

	if len(s) == 2 {
		hour = s[1]
		hs := []string{":", " ", "/", "", "-"}

		format += " "
		//hour
		status, i := Contains(hs, hour)
		if status {

			index, err := strconv.Atoi(i)
			if err != nil {
				panic("error on formatting date time")
			}

			h := strings.Split(hour, string(hs[index]))

			var tmp []string
			for i := 0; i < len(h); i++ {
				switch h[i] {
				case "H":
					tmp = append(tmp, "15")
				case "h":
					tmp = append(tmp, "03")
				case "g":
					tmp = append(tmp, "3")
				case "i":
					tmp = append(tmp, "04")
				case "s":
					tmp = append(tmp, "05")
				}
			}

			format += strings.Join(tmp, hs[index])
		}
	}

	var result string
	if data.Data != nil {
		result = data.Data.(time.Time).Format(format)
	} else {
		rn := time.Now()
		result = rn.Format(format)
	}

	epoch, err := time.Parse(format, result)
	if err != nil {
		panic("error on converting to epoch")
	}

	return result, epoch.Unix()
}

/**
-- UPDATE V0.1.7 --

GET DPP PAJAK
*/

func RumusPajak(tanggal string, tipe string) float64 {
	// Use current date if no date is provided
	var tgl string
	if tanggal == "" {
		tgl, _ = DateFormat(DateTime{Format: "Y-m-d"})
	} else {
		tgl = tanggal
	}

	// Validate date format
	_, err := time.Parse("2006-01-02", tgl)
	if err != nil {
		return 0
	}

	var pajak float64
	var tanggalBatas map[string]float64

	if tipe == "ppn" {
		pajak = 11.0 // Default tax rate
		tanggalBatas = map[string]float64{
			"2022-04-01": 11.0,
			// "2025-02-01": 1.0,
			// "2026-01-01": 1.0,
		}

	} else {

		pajak = 1.0 // Default tax rate
		tanggalBatas = map[string]float64{
			"2025-01-01": 11.0 / 12.0,
			// "2025-02-01": 1.0,
			// "2026-01-01": 1.0,
		}
	}

	// Sort batas dates in descending order
	var batasDates []string
	for batas := range tanggalBatas {
		batasDates = append(batasDates, batas)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(batasDates)))

	// Find the correct tax rate
	parsedTanggal, _ := time.Parse("2006-01-02", tgl)
	for _, batas := range batasDates {
		parsedBatas, _ := time.Parse("2006-01-02", batas)
		if parsedTanggal.After(parsedBatas) || parsedTanggal.Equal(parsedBatas) {
			pajak = tanggalBatas[batas]
			break
		}
	}

	return pajak
}

func IsEmpty(data []any) bool {

	if len(data) > 0 {
		return true
	} else {
		return false
	}
}

func EmptyString(data interface{}) bool {

	x := data.(string)

	if x == "" {
		return true
	} else {
		return false
	}
}

/** ----- UPDATE V0.3.0 YEAH -------- */
func Ranum() int {
	randomizer := ranmath.New(ranmath.NewSource(time.Now().UTC().UnixNano()))
	return randomizer.Int()
}

func HtmlTo(tipe, filepath, temp string, data map[string]interface{}) string {

	tmpl, err := template.ParseFiles("templates" + temp)
	if err != nil {
		log.Fatal("Error parsing template:", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		log.Fatal("Error executing template:", err)
	}

	htmlPath := "./files/output.html"
	err = os.WriteFile(htmlPath, buf.Bytes(), 0644)
	if err != nil {
		log.Fatal("Error saving HTML:", err)
	}

	path := "./files" + filepath

	switch tipe {
	case "html":
		data, err := os.ReadFile(htmlPath)

		if err != nil {
			log.Fatalf("gagal baca html")
		}

		filepath = string(data)

	case "jpg":
		cmd := exec.Command("wkhtmltoimage", htmlPath, path)

		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Gagal membuat Image: %v\nOutput: %s", err, out)
		}

		log.Println("✅ Image berhasil dibuat:", path)
	default:
		cmd := exec.Command("wkhtmltopdf", htmlPath, path)

		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Gagal membuat PDF: %v\nOutput: %s", err, out)
		}

		log.Println("✅ PDF berhasil dibuat:", path)
	}

	return filepath
}

func SaveExcel(f *excelize.File, path string, filename string) (bool, string) {

	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModeDir|0755)
		if err != nil {
			return false, "Error creating directory: " + err.Error()
		}
	}

	if err := f.SaveAs(fmt.Sprintf("%s/%s", path, filename)); err != nil {
		return false, err.Error()
	}

	return true, ""
}
