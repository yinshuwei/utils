package utils

import (
	"io"
	"log"
	"os"
	"strings"
)

var LogOutput io.Writer = os.Stdout
var LogFlag int = 0

/**
 * defer utils.SetLogOutPut("logs/logfile.txt")()
 **/
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

	if err != nil {
		panic(err)
	}

	LogOutput = f
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
				} else {
					return os.Mkdir(dir, os.ModePerm)
				}
			} else {
				return err
			}
		}
	}
	return nil
}

func SetLogFlags(flag int) {
	LogFlag = flag
	log.SetFlags(flag)
}
