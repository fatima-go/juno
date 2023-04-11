//
// Copyright (c) 2018 SK TECHX.
// All right reserved.
//
// This software is the confidential and proprietary information of SK TECHX.
// You shall not disclose such Confidential Information and
// shall use it only in accordance with the terms of the license agreement
// you entered into with SK TECHX.
//
//
// @project juno
// @author 1100282
// @date 2018. 3. 10. PM 7:52
//

package service

import (
	"throosea.com/fatima"
	"throosea.com/juno/infra"
)

var inspector infra.SystemInspector

func PrepareService(fatimaRuntime fatima.FatimaRuntime) {
	inspector = infra.NewSystemInspector(fatimaRuntime)
	newProcessMonitor(fatimaRuntime)
}
