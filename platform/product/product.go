/*
 * Copyright 2021-2024 JetBrains s.r.o.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package product

import (
	"github.com/JetBrains/qodana-cli/v2025/platform/strutil"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	ReleaseVersion = "2025.1"
	ShortVersion   = "251"
	IsReleased     = true

	EapSuffix  = "-EAP"
	ReleaseVer = "release"
	EapVer     = "eap"

	QDJVMC = "QDJVMC"
	QDJVM  = "QDJVM"
	QDAND  = "QDAND"
	QDPHP  = "QDPHP"
	QDPY   = "QDPY"
	QDPYC  = "QDPYC"
	QDJS   = "QDJS"
	QDGO   = "QDGO"
	QDNET  = "QDNET"
	QDNETC = "QDNETC"
	QDANDC = "QDANDC"
	QDRST  = "QDRST"
	QDRUBY = "QDRUBY"
	QDCLC  = "QDCLC"
	QDCPP  = "QDCPP"
)

var (
	VersionsMap = map[string]string{
		ReleaseVer: "2025.1",
		EapVer:     "2024.3",
	}

	Products = map[string]string{
		QDJVM:  "IIU",
		QDJVMC: "IIC",
		// QDAND: // don't use it right now
		// QDANDC: // don't use it right now
		QDPHP:  "PS",
		QDJS:   "WS",
		QDNET:  "RD",
		QDPY:   "PCP",
		QDPYC:  "PCC",
		QDGO:   "GO",
		QDRUBY: "RM",
		QDRST:  "RR",
		QDCPP:  "CL",
	}

	DockerImageMap = map[string]string{
		QDAND:  "jetbrains/qodana-android:",
		QDANDC: "jetbrains/qodana-jvm-android:",
		QDPHP:  "jetbrains/qodana-php:",
		QDJS:   "jetbrains/qodana-js:",
		QDNET:  "jetbrains/qodana-dotnet:",
		QDCPP:  "jetbrains/qodana-cpp:",
		QDNETC: "jetbrains/qodana-cdnet:",
		QDPY:   "jetbrains/qodana-python:",
		QDPYC:  "jetbrains/qodana-python-community:",
		QDGO:   "jetbrains/qodana-go:",
		QDJVM:  "jetbrains/qodana-jvm:",
		QDJVMC: "jetbrains/qodana-jvm-community:",
		QDCLC:  "jetbrains/qodana-clang:",
		QDRUBY: "jetbrains/qodana-ruby:",
		//QDRST:  "jetbrains/qodana-rust:",
	}

	// AllNativeCodes is a list of all supported Qodana linters product codes
	AllNativeCodes = []string{QDNET, QDJVM, QDJVMC, QDGO, QDPY, QDPYC, QDJS, QDPHP}
)

func Image(code string) string {
	if val, ok := DockerImageMap[code]; ok {
		if //goland:noinspection GoBoolExpressions
		!IsReleased {
			return val + ReleaseVersion + "-eap"
		}
		if code == QDNETC || code == QDCLC || code == QDRUBY || code == QDCPP {
			return val + ReleaseVersion + "-eap"
		}
		return val + ReleaseVersion
	} else {
		log.Fatal("Unknown code: " + code)
		return ""
	}
}

// LangsProductCodes is a map of languages to linters.
var LangsProductCodes = map[string][]string{
	"Java":              {QDJVM, QDJVMC, QDAND, QDANDC},
	"Kotlin":            {QDJVM, QDJVMC, QDAND, QDANDC},
	"PHP":               {QDPHP},
	"Python":            {QDPY, QDPYC},
	"JavaScript":        {QDJS},
	"TypeScript":        {QDJS},
	"Go":                {QDGO},
	"C#":                {QDNET, QDNETC},
	"F#":                {QDNET},
	"Visual Basic .NET": {QDNET, QDNETC},
	"C":                 {QDCPP, QDCLC, QDNET},
	"C++":               {QDCPP, QDCLC, QDNET},
	"Ruby":              {QDRUBY},
}

var AllSupportedPaidCodes = []string{QDJVM, QDPHP, QDPY, QDJS, QDGO, QDNET, QDAND, QDCPP, QDRUBY}
var AllSupportedFreeCodes = []string{QDJVMC, QDPYC, QDANDC, QDNETC, QDCLC}

var AllFixesSupportedProducts = []string{QDJVM, QDNET, QDPY, QDJS, QDPHP, QDGO, QDAND, QDRUBY}

func allImages(codes []string) []string {
	var images []string
	for _, code := range codes {
		images = append(images, Image(code))
	}
	return images
}

var AllSupportedFreeImages = allImages(AllSupportedFreeCodes)

// AllImages is a list of all supported linters.
var AllImages = append(allImages(AllSupportedPaidCodes), AllSupportedFreeImages...)

// AllCodes is a list of codes for all supported linters.
var AllCodes = append(AllSupportedPaidCodes, AllSupportedFreeCodes...)

func GuessProductCode(ide string, linter string) string {
	if ide != "" {
		productCode := strings.TrimSuffix(ide, EapSuffix)
		if _, ok := Products[productCode]; ok {
			return productCode
		}
		return ""
	} else if linter != "" {
		// if Linter contains registry.jetbrains.team/p/sa/containers/ or https://registry.jetbrains.team/p/sa/containers/
		// then replace it with jetbrains/ and do the comparison
		linter := strings.TrimPrefix(linter, "https://")
		if strings.HasPrefix(linter, "registry.jetbrains.team/p/sa/containers/") {
			linter = strings.TrimPrefix(linter, "registry.jetbrains.team/p/sa/containers/")
			linter = "jetbrains/" + linter
		}
		for k, v := range DockerImageMap {
			if strings.HasPrefix(linter, v) {
				return k
			}
		}
	}
	return ""
}

func IsNativeAnalyzer(analyzer string) bool {
	return strutil.Contains(AllNativeCodes, analyzer)
}
