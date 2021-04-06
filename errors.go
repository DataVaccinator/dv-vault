package main

/*
code 	desc 					status
1 		Missing Parameters. 	INVALID
2 		Wrong Protocol. 	INVALID
3 		Your software seems outdated. 	INVALID
4 		The account was locked due to possible misuse. 	INVALID
5 		Invalid credentials (check sid and spwd). 	INVALID
6 		Invalid encoding (check data values and JSON integrity). 	INVALID
7 		Not found (vid is not found in the system). 	INVALID
8 		Invalid partner (you are not allowed to access foreign data). 	INVALID
9 		Invalid parameter size (some parameter exceeds limits). 	INVALID
99 		Some internal service error happened. Please contact support. 	ERROR
*/

const (
	DV_MISSING_PARAM        = 1
	DV_WRONG_PROTOCOL       = 2
	DV_OUTDATED             = 3
	DV_LOCKED               = 4
	DV_INVALID_CREDENTIALS  = 5
	DV_INVALID_ENCODING     = 6
	DV_VID_NOT_FOUND        = 7
	DV_INVALID_PARTNER      = 8
	DV_INVALID_PARAMSIZE    = 9
	DV_VID_DELETED          = 10
	DV_PLUGIN_INVALID_CALL  = 20
	DV_PLUGIN_MISSING_PARAM = 21
	DV_INTERNAL_ERROR       = 99
)
