package main

// FipsToIso maps GDELT's FIPS 10-4 country codes to ISO 3166-1 alpha-3 codes.
// Source: https://github.com/mysociety/gaze/blob/master/data/fips-10-4-to-iso-country-codes.csv
// Only sovereign/UN-recognized states are included; territories and
// entries with no valid ISO code are intentionally omitted.
var FipsToIso = map[string]string{
	"AF": "AFG", // Afghanistan
	"AL": "ALB", // Albania
	"AG": "DZA", // Algeria
	"AN": "AND", // Andorra
	"AO": "AGO", // Angola
	"AC": "ATG", // Antigua and Barbuda
	"AR": "ARG", // Argentina
	"AM": "ARM", // Armenia
	"AS": "AUS", // Australia
	"AU": "AUT", // Austria
	"AJ": "AZE", // Azerbaijan
	"BF": "BHS", // Bahamas
	"BA": "BHR", // Bahrain
	"BG": "BGD", // Bangladesh
	"BB": "BRB", // Barbados
	"BO": "BLR", // Belarus
	"BE": "BEL", // Belgium
	"BH": "BLZ", // Belize
	"BN": "BEN", // Benin
	"BT": "BTN", // Bhutan
	"BL": "BOL", // Bolivia
	"BK": "BIH", // Bosnia and Herzegovina
	"BC": "BWA", // Botswana
	"BR": "BRA", // Brazil
	"BX": "BRN", // Brunei
	"BU": "BGR", // Bulgaria
	"UV": "BFA", // Burkina Faso
	"BM": "MMR", // Burma/Myanmar
	"BY": "BDI", // Burundi
	"CB": "KHM", // Cambodia
	"CM": "CMR", // Cameroon
	"CA": "CAN", // Canada
	"CV": "CPV", // Cape Verde
	"CT": "CAF", // Central African Republic
	"CD": "TCD", // Chad
	"CI": "CHL", // Chile
	"CH": "CHN", // China
	"CO": "COL", // Colombia
	"CN": "COM", // Comoros
	"CG": "COD", // Congo, Democratic Republic of the
	"CF": "COG", // Congo, Republic of the
	"CS": "CRI", // Costa Rica
	"IV": "CIV", // Cote d'Ivoire
	"HR": "HRV", // Croatia
	"CU": "CUB", // Cuba
	"UC": "CUW", // Curacao
	"CY": "CYP", // Cyprus
	"EZ": "CZE", // Czech Republic
	"DA": "DNK", // Denmark
	"DJ": "DJI", // Djibouti
	"DO": "DMA", // Dominica
	"DR": "DOM", // Dominican Republic
	"EC": "ECU", // Ecuador
	"EG": "EGY", // Egypt
	"ES": "SLV", // El Salvador
	"EK": "GNQ", // Equatorial Guinea
	"ER": "ERI", // Eritrea
	"EN": "EST", // Estonia
	"ET": "ETH", // Ethiopia
	"FJ": "FJI", // Fiji
	"FI": "FIN", // Finland
	"FR": "FRA", // France
	"GB": "GAB", // Gabon
	"GA": "GMB", // Gambia
	"GG": "GEO", // Georgia
	"GM": "DEU", // Germany
	"GH": "GHA", // Ghana
	"GR": "GRC", // Greece
	"GJ": "GRD", // Grenada
	"GT": "GTM", // Guatemala
	"GV": "GIN", // Guinea
	"PU": "GNB", // Guinea-Bissau
	"GY": "GUY", // Guyana
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
	"JM": "JAM", // Jamaica
	"JA": "JPN", // Japan
	"JO": "JOR", // Jordan
	"KZ": "KAZ", // Kazakhstan
	"KE": "KEN", // Kenya
	"KR": "KIR", // Kiribati
	"KN": "PRK", // Korea, North
	"KS": "KOR", // Korea, South
	"KV": "XKX", // Kosovo (user-assigned ISO)
	"KU": "KWT", // Kuwait
	"KG": "KGZ", // Kyrgyzstan
	"LA": "LAO", // Laos
	"LG": "LVA", // Latvia
	"LE": "LBN", // Lebanon
	"LT": "LSO", // Lesotho
	"LI": "LBR", // Liberia
	"LY": "LBY", // Libya
	"LS": "LIE", // Liechtenstein
	"LH": "LTU", // Lithuania
	"LU": "LUX", // Luxembourg
	"MK": "MKD", // Macedonia (North Macedonia)
	"MA": "MDG", // Madagascar
	"MI": "MWI", // Malawi
	"MY": "MYS", // Malaysia
	"MV": "MDV", // Maldives
	"ML": "MLI", // Mali
	"MT": "MLT", // Malta
	"RM": "MHL", // Marshall Islands
	"MR": "MRT", // Mauritania
	"MP": "MUS", // Mauritius
	"MX": "MEX", // Mexico
	"FM": "FSM", // Micronesia, Federated States of
	"MD": "MDA", // Moldova
	"MN": "MCO", // Monaco
	"MG": "MNG", // Mongolia
	"MJ": "MNE", // Montenegro
	"MO": "MAR", // Morocco
	"MZ": "MOZ", // Mozambique
	"WA": "NAM", // Namibia
	"NR": "NRU", // Nauru
	"NP": "NPL", // Nepal
	"NL": "NLD", // Netherlands
	"NZ": "NZL", // New Zealand
	"NU": "NIC", // Nicaragua
	"NG": "NER", // Niger
	"NI": "NGA", // Nigeria
	"NO": "NOR", // Norway
	"MU": "OMN", // Oman
	"PK": "PAK", // Pakistan
	"PS": "PLW", // Palau
	"PM": "PAN", // Panama
	"PP": "PNG", // Papua New Guinea
	"PA": "PRY", // Paraguay
	"PE": "PER", // Peru
	"RP": "PHL", // Philippines
	"PL": "POL", // Poland
	"PO": "PRT", // Portugal
	"QA": "QAT", // Qatar
	"RO": "ROU", // Romania
	"RS": "RUS", // Russia
	"RW": "RWA", // Rwanda
	"SC": "KNA", // Saint Kitts and Nevis
	"ST": "LCA", // Saint Lucia
	"VC": "VCT", // Saint Vincent and the Grenadines
	"WS": "WSM", // Samoa
	"SM": "SMR", // San Marino
	"TP": "STP", // Sao Tome and Principe
	"SA": "SAU", // Saudi Arabia
	"SG": "SEN", // Senegal
	"RI": "SRB", // Serbia
	"SE": "SYC", // Seychelles
	"SL": "SLE", // Sierra Leone
	"SN": "SGP", // Singapore
	"LO": "SVK", // Slovakia
	"SI": "SVN", // Slovenia
	"BP": "SLB", // Solomon Islands
	"SO": "SOM", // Somalia
	"SF": "ZAF", // South Africa
	"OD": "SSD", // South Sudan
	"SP": "ESP", // Spain
	"CE": "LKA", // Sri Lanka
	"SU": "SDN", // Sudan
	"NS": "SUR", // Suriname
	"WZ": "SWZ", // Swaziland (Eswatini)
	"SW": "SWE", // Sweden
	"SZ": "CHE", // Switzerland
	"SY": "SYR", // Syria
	"TW": "TWN", // Taiwan
	"TI": "TJK", // Tajikistan
	"TZ": "TZA", // Tanzania
	"TH": "THA", // Thailand
	"TT": "TLS", // Timor-Leste
	"TO": "TGO", // Togo
	"TN": "TON", // Tonga
	"TD": "TTO", // Trinidad and Tobago
	"TS": "TUN", // Tunisia
	"TU": "TUR", // Turkey
	"TX": "TKM", // Turkmenistan
	"TV": "TUV", // Tuvalu
	"UG": "UGA", // Uganda
	"UP": "UKR", // Ukraine
	"AE": "ARE", // United Arab Emirates
	"UK": "GBR", // United Kingdom
	"US": "USA", // United States
	"UY": "URY", // Uruguay
	"UZ": "UZB", // Uzbekistan
	"NH": "VUT", // Vanuatu
	"VT": "VAT", // Vatican City
	"VE": "VEN", // Venezuela
	"VM": "VNM", // Vietnam
	"YM": "YEM", // Yemen
	"ZA": "ZMB", // Zambia
	"ZI": "ZWE", // Zimbabwe
}

// IsoToFips is the reverse of FipsToIso, built once at startup.
// Use this for O(1) lookups instead of scanning FipsToIso each time.
var IsoToFips map[string]string

func init() {
	IsoToFips = make(map[string]string, len(FipsToIso))
	for fips, iso := range FipsToIso {
		IsoToFips[iso] = fips
	}
}
