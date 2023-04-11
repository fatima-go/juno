/*
 * Copyright (c) 2018 throosea.com.
 * All right reserved.
 *
 * This software is the confidential and proprietary information of throosea.com.
 * You shall not disclose such Confidential Information and
 * shall use it only in accordance with the terms of the license agreement
 * you entered into with throosea.com.
 */

package service

import (
	"fmt"
	"testing"
	"time"
)

var countryTz = map[string]string{
	"Hungary": "Europe/Budapest",
	"Egypt":   "Africa/Cairo",
}

func Test(t *testing.T) {
	utc := time.UTC
	utc2, _ := time.LoadLocation("UTC")
	fmt.Printf("local : %s\n", time.Local)
	if utc == utc2 {
		fmt.Printf("seoul is same to local\n")
	} else {
		fmt.Printf("seoul is NOT same to local\n")
	}

}
