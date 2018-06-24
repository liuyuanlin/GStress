package main

import (
	"GStress/logger"
	"fmt"

	"github.com/360EntSecGroup-Skylar/excelize"
)

type ExcelRow map[string]string
type ExcelCfg struct {
	MExcelRows []ExcelRow
}

func (E *ExcelCfg) Parser(cfgFile string, sheetName string) error {
	logger.Log4.Debug("<ENTER> :cfgFile:%s, sheetName:%s", cfgFile, sheetName)
	defer logger.Log4.Debug("<LEAVE>")
	var lRetErr error
	xlsx, lRetErr := excelize.OpenFile(cfgFile)
	if lRetErr != nil {
		logger.Log4.Debug("<ENTER> :cfgFile:%s", lRetErr)
		return lRetErr
	}

	// Get all the rows in the Sheet1.
	rows := xlsx.GetRows(sheetName)
	for _, row := range rows {
		for _, colCell := range row {
			fmt.Print(colCell, "\t")
		}
		fmt.Println()
	}

	//读取key
	var rowCount int = 0

	for _, row := range rows {
		//第一行作为key
		if rowCount == 0 {
			rowCount = rowCount + 1
			continue
		}
		rowValue := make(ExcelRow)
		cellCount := 0
		for _, colCell := range row {
			rowValue[rows[0][cellCount]] = colCell
			cellCount++
		}
		logger.Log4.Debug("rowValue:%v", rowValue)
		E.MExcelRows = append(E.MExcelRows, rowValue)
		fmt.Println()
	}
	logger.Log4.Debug("E.MExcelRows:%v", E.MExcelRows)

	return lRetErr
}
