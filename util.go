package main

import "log"

func debugf(str string, params ...interface{}) {
	if debug {
		if str[len(str)-1] != '\n' {
			str += "\n"
		}

		str = "[DEBUG] " + str

		log.Printf(str, params...)
	}
}

func warnf(str string, params ...interface{}) {
	if debug {
		if str[len(str)-1] != '\n' {
			str += "\n"
		}

		str = "[WARNING] " + str

		log.Printf(str, params...)
	}
}
