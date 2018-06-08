package utils

import (
	"io"
	"log"
	"os"
	"strings"
)

var logOutput io.Writer = os.Stdout
var logFlag int

// SetLogOutPut SetLogOutPut
// defer utils.SetLogOutPut("logs/logfile.txt")()
func SetLogOutPut(path string) func() {
	path = strings.Replace(path, "\\", "/", -1)

	_, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			err = checkAndMkParentDir(path)

			if err != nil {
				panic(err)
			}

			ff, err := os.Create(path)
			if err != nil {
				panic(err)
			}
			ff.Close()
		}
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	logOutput = f
	log.SetOutput(f)

	return func() {
		f.Close()
	}
}

/**
 * parent dir is exist
 * if exist return nil
 * if not exist mkdir and return nil
 * if create err return the error
 **/
func checkAndMkParentDir(path string) error {
	if strings.Contains(path, "/") {
		dir := path[0:strings.LastIndex(path, "/")]
		_, err := os.Open(dir)
		if err != nil {
			if os.IsNotExist(err) {
				err := checkAndMkParentDir(dir)
				if err != nil {
					return err
				}
				return os.Mkdir(dir, os.ModePerm)
			}
			return err
		}
	}
	return nil
}

// SetLogFlags SetLogFlags
func SetLogFlags(flag int) {
	logFlag = flag
	log.SetFlags(flag)
}
