package pidp11

const ANTI_GHOSTING_PAUSE_NS = 1e4
const LEDS_COUNT = 72

var LED_ROWS = [...]uint{20, 21, 22, 23, 24, 25}
var ROWS = [...]uint{16, 17, 18}
var COLS = [...]uint{26, 27, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}

const (
	LED_A0 LedID = iota
	LED_A1
	LED_A2
	LED_A3
	LED_A4
	LED_A5
	LED_A6
	LED_A7
	LED_A8
	LED_A9
	LED_A10
	LED_A11
	LED_A12
	LED_A13
	LED_A14
	LED_A15
	LED_A16
	LED_A17
	LED_A18
	LED_A19
	LED_A20
	LED_A21
	LED_UNUSED1
	LED_UNUSED2
	LED_ADDR_22
	LED_ADDR_18
	LED_ADDR_16
	LED_DATA
	LED_KERNEL
	LED_SUPER
	LED_USER
	LED_MASTER
	LED_PAUSE
	LED_RUN
	LED_ADRS_ERR
	LED_PAR_ERR
	LED_D0
	LED_D1
	LED_D2
	LED_D3
	LED_D4
	LED_D5
	LED_D6
	LED_D7
	LED_D8
	LED_D9
	LED_D10
	LED_D11
	LED_D12
	LED_D13
	LED_D14
	LED_D15
	LED_PAR_LO
	LED_PAR_HI
	LED_USER_D
	LED_SUPER_D
	LED_KERNEL_D
	LED_CONS_PHY
	LED_DATA_PATHS
	LED_BUS_REG
	LED_UNUSED3
	LED_UNUSED4
	LED_UNUSED5
	LED_UNUSED6
	LED_UNUSED7
	LED_UNUSED8
	LED_USER_I
	LED_SUPER_I
	LED_KERNEL_I
	LED_PROG_PHY
	LED_μADR_FPP_CPU
	LED_DISPLAY_REGISTER
)

var LED_NAMES = []string{
	"A0",
	"A1",
	"A2",
	"A3",
	"A4",
	"A5",
	"A6",
	"A7",
	"A8",
	"A9",
	"A10",
	"A11",
	"A12",
	"A13",
	"A14",
	"A15",
	"A16",
	"A17",
	"A18",
	"A19",
	"A20",
	"A21",
	"UNUSED1",
	"UNUSED2",
	"ADDR_22",
	"ADDR_18",
	"ADDR_16",
	"DATA",
	"KERNEL",
	"SUPER",
	"USER",
	"MASTER",
	"PAUSE",
	"RUN",
	"ADRS_ERR",
	"PAR_ERR",
	"D0",
	"D1",
	"D2",
	"D3",
	"D4",
	"D5",
	"D6",
	"D7",
	"D8",
	"D9",
	"D10",
	"D11",
	"D12",
	"D13",
	"D14",
	"D15",
	"PAR_LO",
	"PAR_HI",
	"USER_D",
	"SUPER_D",
	"KERNEL_D",
	"CONS_PHY",
	"DATA_PATHS",
	"BUS_REG",
	"UNUSED3",
	"UNUSED4",
	"UNUSED5",
	"UNUSED6",
	"UNUSED7",
	"UNUSED8",
	"USER_I",
	"SUPER_I",
	"KERNEL_I",
	"PROG_PHY",
	"μADR_FPP_CPU",
	"DISPLAY_REGISTER",
}

func LedName(id LedID) string {
	return LED_NAMES[int(id)]
}

func LedIDByName(ledName string) (LedID, bool) {
	for i, name := range LED_NAMES {
		if name == ledName {
			return LedID(i), true
		}
	}
	return LED_UNUSED1, false
}

const (
	SW_SR0 SwitchID = iota
	SW_SR1
	SW_SR2
	SW_SR3
	SW_SR4
	SW_SR5
	SW_SR6
	SW_SR7
	SW_SR8
	SW_SR9
	SW_SR10
	SW_SR11
	SW_SR12
	SW_SR13
	SW_SR14
	SW_SR15
	SW_SR16
	SW_SR17
	SW_SR18
	SW_SR19
	SW_SR20
	SW_SR21
	SW_KNOBA_PUSH
	SW_KNOBD_PUSH
	SW_TEST
	SW_LOAD
	SW_EXAM
	SW_DEP
	SW_CONT
	SW_ENABLE
	SW_SINST
	SW_START
	// Physical switches, semi-random names, not emitted
	SW_KNOBA_ACW
	SW_KNOBA_CW
	SW_KNOBD_ACW
	SW_KNOBD_CW
	// Synthetic switches, emitted with state true for clockwise
	SW_KNOBA
	SW_KNOBD
	SW_NONE
)

var SWITCH_NAMES = []string{
	"SR0",
	"SR1",
	"SR2",
	"SR3",
	"SR4",
	"SR5",
	"SR6",
	"SR7",
	"SR8",
	"SR9",
	"SR10",
	"SR11",
	"SR12",
	"SR13",
	"SR14",
	"SR15",
	"SR16",
	"SR17",
	"SR18",
	"SR19",
	"SR20",
	"SR21",
	"KNOBA_PUSH",
	"KNOBD_PUSH",
	"TEST",
	"LOAD",
	"EXAM",
	"DEP",
	"CONT",
	"ENABLE",
	"SINST",
	"START",
	"KNOBA_ACW",
	"KNOBA_CW",
	"KNOBD_ACW",
	"KNOBD_CW",
	"KNOBA",
	"KNOBD",
	"NONE",
}

func SwitchName(id SwitchID) string {
	return SWITCH_NAMES[id]
}

const BRIGHTNESS_STEPS = 32

// From 07.1_blinkenlight_server/iopattern.c
var BRIGHTNESS_PHASES = [32][31]bool{
	{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false}, //  0/31 =  0%
	{true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},  //  1/31 =  3%
	{true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false},   //  2/31 =  6%
	{true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false},    //  3/31 = 10%
	{true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false},     //  4/31 = 13%
	{true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, false, false, false, false, false, false, false, false, false, false, false},      //  5/31 = 16%
	{true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false},       //  6/31 = 19%
	{true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, false, false, false, false, false, false, false, false, false, false, false},        //  7/31 = 23%
	{true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false},         //  8/31 = 26%
	{true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, false, false, false, false, false, false, false, false, false, false},          //  9/31 = 29%
	{true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false},           // 10/31 = 32%
	{true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, true, false, false, false, false, false, false, false, false, false},            // 11/31 = 35%
	{true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false},             // 12/31 = 39%
	{true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false},              // 13/31 = 42%
	{true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false},               // 14/31 = 45%
	{true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, true, true, true, true, true, true, true, false, false, false, false, false, false, false, false},                // 15/31 = 48%
	{true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, false, false, false, false, false, false, false},                 // 16/31 = 52%
	{true, true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, false, false, false, false, false, false},                  // 17/31 = 55%
	{true, true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, true, false, false, false, false, false, false},                   // 18/31 = 58%
	{true, true, true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, true, false, false, false, false, false},                    // 19/31 = 61%
	{true, true, true, true, true, true, true, true, true, true, false, false, false, false, false, false, true, true, true, true, true, true, true, true, true, true, false, false, false, false, false},                     // 20/31 = 65%
	{true, true, true, true, true, true, true, true, true, true, true, false, false, false, false, false, true, true, true, true, true, true, true, true, true, true, false, false, false, false, false},                      // 21/31 = 68%
	{true, true, true, true, true, true, true, true, true, true, true, false, false, false, false, false, true, true, true, true, true, true, true, true, true, true, true, false, false, false, false},                       // 22/31 = 71%
	{true, true, true, true, true, true, true, true, true, true, true, true, false, false, false, false, true, true, true, true, true, true, true, true, true, true, true, false, false, false, false},                        // 23/31 = 74%
	{true, true, true, true, true, true, true, true, true, true, true, true, false, false, false, false, true, true, true, true, true, true, true, true, true, true, true, true, false, false, false},                         // 24/31 = 77%
	{true, true, true, true, true, true, true, true, true, true, true, true, true, false, false, false, true, true, true, true, true, true, true, true, true, true, true, true, false, false, false},                          // 25/31 = 81%
	{true, true, true, true, true, true, true, true, true, true, true, true, true, false, false, false, true, true, true, true, true, true, true, true, true, true, true, true, true, false, false},                           // 26/31 = 84%
	{true, true, true, true, true, true, true, true, true, true, true, true, true, true, false, false, true, true, true, true, true, true, true, true, true, true, true, true, true, false, false},                            // 27/31 = 87%
	{true, true, true, true, true, true, true, true, true, true, true, true, true, true, false, false, true, true, true, true, true, true, true, true, true, true, true, true, true, true, false},                             // 28/31 = 90%
	{true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, false, true, true, true, true, true, true, true, true, true, true, true, true, true, true, false},
	// 29/31 = 94%
	{true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, false, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true}, // 30/31 = 97%
	{true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true},  // 31/31 = 100%
}
