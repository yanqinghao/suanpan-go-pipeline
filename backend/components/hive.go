package components

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xuelang-group/suanpan-go-sdk/config"
	"github.com/xuelang-group/suanpan-go-sdk/suanpan/v1/log"
	"github.com/xuelang-group/suanpan-go-sdk/suanpan/v1/storage"
	_ "sqlflow.org/gohive"
)

type hiveDataCol struct {
	Name string
	Type string
}

func hiveReaderMain(currentNode Node, inputData RequestData) (map[string]interface{}, error) {
	hiveConn := fmt.Sprintf("%s:%s@%s:%s/%s?auth=PLAIN",
		currentNode.Config["user"].(string),
		url.QueryEscape(currentNode.Config["password"].(string)),
		currentNode.Config["host"].(string),
		currentNode.Config["port"].(string),
		currentNode.Config["dbname"].(string))
	db, err := sql.Open("hive", hiveConn)
	if err != nil {
		log.Info("数据库连接失败，请检查配置")
		return map[string]interface{}{}, nil
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Info("数据库测试连接失败，请检查配置")
		return map[string]interface{}{}, nil
	}
	tableCols := make([]hiveDataCol, 0)
	tableQueryStr := ""
	if len(currentNode.Config["sql"].(string)) == 0 {
		tableName := loadParameter(currentNode.Config["table"].(string), currentNode.InputData)
		tableQueryStr = fmt.Sprintf("SELECT * FROM %s", tableName)
	} else {
		tableQueryStr = loadParameter(currentNode.Config["sql"].(string), currentNode.InputData)
	}
	rows, err := db.Query(tableQueryStr)
	if err != nil {
		log.Info("数据表检索失败")
		return map[string]interface{}{}, nil
	}
	columnNames, err := rows.Columns()
	if err != nil {
		log.Info("查询数据表结构失败")
		return map[string]interface{}{}, nil
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		log.Info("查询数据表类型失败")
		return map[string]interface{}{}, nil
	}
	for i, col := range columnNames {
		tableCol := hiveDataCol{Name: col, Type: columnTypes[i].DatabaseTypeName()}
		tableCols = append(tableCols, tableCol)
	}
	records := make([][]string, 0)
	headers := make([]string, 0)
	headers = append(headers, "indexCol")
	for _, col := range tableCols {
		headers = append(headers, col.Name)
	}
	records = append(records, headers)
	recordNum := 0
	defer rows.Close()
	for rows.Next() {
		record := make([]interface{}, 0, len(tableCols))
		for _, col := range tableCols {
			switch strings.ToLower(col.Type) {
			case "date", "time without time zone", "time with time zone", "timestamp without time zone", "timestamp with time zone":
				record = append(record, sql.NullTime{})
			default:
				record = append(record, sql.NullString{})
			}
		}
		recordP := make([]interface{}, len(tableCols))
		for i := range record {
			recordP[i] = &record[i]
		}
		err = rows.Scan(recordP...)
		if err != nil {
			log.Info("数据表数据检索失败")
			return map[string]interface{}{}, nil
		}
		data := make([]string, 0)
		data = append(data, strconv.FormatInt(int64(recordNum), 10))
		for i := range record {
			switch v := record[i].(type) {
			case int64, int16, int32, int8, int, uint, uint16, uint32, uint64:
				data = append(data, strconv.FormatInt(v.(int64), 10))
			case bool:
				data = append(data, strconv.FormatBool(v))
			case float32, float64:
				data = append(data, strconv.FormatFloat(v.(float64), 'E', -1, 32))
			case time.Time:
				if strings.ToLower(tableCols[i].Type) == "date" {
					data = append(data, v.Format("2006-01-02"))
				} else {
					data = append(data, v.Format("2006-01-02 15:04:05"))
				}
			case nil:
				data = append(data, "")
			case []uint8:
				data = append(data, string([]byte(v)))
			default:
				data = append(data, v.(string))
			}
		}
		recordNum += 1
		records = append(records, data)
	}
	os.Mkdir(currentNode.Id, os.ModePerm)
	tmpPath := fmt.Sprintf("%s/data.csv", currentNode.Id)
	tmpKey := fmt.Sprintf("studio/%s/tmp/%s/%s/%s/%s", config.GetEnv().SpUserId, config.GetEnv().SpAppId, strings.Join(strings.Split(inputData.ID, "-"), ""), config.GetEnv().SpNodeId, currentNode.Id)
	os.Remove(tmpPath)
	file, err := os.Create(tmpPath)
	if err != nil {
		log.Error("无法创建临时文件")
		return map[string]interface{}{}, nil
	}
	w := csv.NewWriter(file)
	err = w.WriteAll(records)
	if err != nil {
		log.Error("无法写入csv数据")
		return map[string]interface{}{}, nil
	}
	storage.FPutObject(fmt.Sprintf("%s/data.csv", tmpKey), tmpPath)

	return map[string]interface{}{"out1": tmpKey}, nil
}

func hiveExecutorMain(currentNode Node, inputData RequestData) (map[string]interface{}, error) {
	hiveConn := fmt.Sprintf("%s:%s@%s:%s/%s?auth=PLAIN",
		currentNode.Config["user"].(string),
		url.QueryEscape(currentNode.Config["password"].(string)),
		currentNode.Config["host"].(string),
		currentNode.Config["port"].(string),
		currentNode.Config["dbname"].(string))
	db, err := sql.Open("hive", hiveConn)
	if err != nil {
		log.Infof("数据库连接失败，请检查配置")
		return map[string]interface{}{}, nil
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Infof("数据库测试连接失败，请检查配置")
		return map[string]interface{}{}, nil
	}
	tableQueryStr := loadParameter(currentNode.Config["sql"].(string), currentNode.InputData)
	_, err = db.Exec(tableQueryStr)
	if err != nil {
		log.Infof("数据表执行sql语句失败")
		return map[string]interface{}{}, nil
	}
	return map[string]interface{}{"out1": "success"}, nil
}
