package utils

import (
	"log"
	"os"
	"strings"
)

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

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		panic(err)
	}

	if err != nil {
		panic(err)
	}

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
