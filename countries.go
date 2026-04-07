package main

// FipsToIso maps GDELT's FIPS 10-4 country codes to standard ISO 3166-1 alpha-3 codes.
var FipsToIso = map[string]string{
	"AF": "AFG", // Afghanistan
	"AG": "DZA", // Algeria
	"AR": "ARG", // Argentina
	"AS": "AUS", // Australia
	"AU": "AUT", // Austria
	"BA": "BHR", // Bahrain
	"BG": "BGD", // Bangladesh
	"BO": "BLR", // Belarus
	"BE": "BEL", // Belgium
	"BL": "BOL", // Bolivia
	"BR": "BRA", // Brazil
	"CB": "KHM", // Cambodia
	"CA": "CAN", // Canada
	"CI": "CHL", // Chile
	"CH": "CHN", // China
	"CO": "COL", // Colombia
	"CG": "COD", // Dem Rep Congo
	"CU": "CUB", // Cuba
	"CZ": "CZE", // Czechia
	"DA": "DNK", // Denmark
	"EG": "EGY", // Egypt
	"ER": "ERI", // Eritrea
	"ET": "ETH", // Ethiopia
	"FI": "FIN", // Finland
	"FR": "FRA", // France
	"GG": "GEO", // Georgia
	"GM": "DEU", // Germany
	"GR": "GRC", // Greece
	"GT": "GTM", // Guatemala
	"HA": "HTI", // Haiti
	"HO": "HND", // Honduras
	"HU": "HUN", // Hungary
	"IC": "ISL", // Iceland
	"IN": "IND", // India
	"ID": "IDN", // Indonesia
	"IR": "IRN", // Iran
	"IZ": "IRQ", // Iraq
	"EI": "IRL", // Ireland
	"IS": "ISR", // Israel
	"IT": "ITA", // Italy
	"JA": "JPN", // Japan
	"JO": "JOR", // Jordan
	"KZ": "KAZ", // Kazakhstan
	"KE": "KEN", // Kenya
	"KN": "PRK", // North Korea
	"KS": "KOR", // South Korea
	"KU": "KWT", // Kuwait
	"LE": "LBN", // Lebanon
	"LY": "LBY", // Libya
	"MY": "MYS", // Malaysia
	"MX": "MEX", // Mexico
	"MO": "MAR", // Morocco
	"BM": "MMR", // Myanmar
	"NP": "NPL", // Nepal
	"NL": "NLD", // Netherlands
	"NZ": "NZL", // New Zealand
	"NI": "NGA", // Nigeria
	"NO": "NOR", // Norway
	"PK": "PAK", // Pakistan
	"PM": "PAN", // Panama
	"PE": "PER", // Peru
	"RP": "PHL", // Philippines
	"PL": "POL", // Poland
	"PO": "PRT", // Portugal
	"QA": "QAT", // Qatar
	"RO": "ROU", // Romania
	"RS": "RUS", // Russia
	"SA": "SAU", // Saudi Arabia
	"SG": "SEN", // Senegal
	"RI": "SRB", // Serbia
	"SN": "SGP", // Singapore
	"SO": "SOM", // Somalia
	"SF": "ZAF", // South Africa
	"SP": "ESP", // Spain
	"CE": "LKA", // Sri Lanka
	"SU": "SDN", // Sudan
	"SW": "SWE", // Sweden
	"SZ": "CHE", // Switzerland
	"SY": "SYR", // Syria
	"TW": "TWN", // Taiwan
	"TH": "THA", // Thailand
	"TU": "TUR", // Turkey
	"UP": "UKR", // Ukraine
	"AE": "ARE", // United Arab Emirates
	"UK": "GBR", // United Kingdom
	"US": "USA", // United States
	"VE": "VEN", // Venezuela
	"VM": "VNM", // Vietnam
	"YM": "YEM", // Yemen
	"ZI": "ZWE", // Zimbabwe
}
