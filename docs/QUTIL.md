# Namespace QUTIL

## QUTIL::MATCH(haystack, needle)

*Retrieve a value using a regular expression (case sensitive)*

If the pattern contains subgroups, the last matching subgroup is returned - otherwise the full match is returned.

Call | Result
---- | ------
`QUTIL::MATCH('abc 123 def', '\d+')` | `123` (full match)
`QUTIL::MATCH('abc 123 def', '\w+\s(\d+)\s+\w+')` | `123` (subgroup match)


## QUTIL::IMATCH(haystack, pattern)

*Retrieve value using a regular expression (case insensitive)*

Just like QUTIL::MATCH(), but case insensitive.


## QUTIL::DATEFMT(timestamp)

*A short function to create a human readable date from a timestamp without remembering the correct format string*

This allows to write
```
QUTIL::DATEFMT(devicetime)
```
instead of
```
DATEFORMAT(devicetime, 'yyyy-MM-dd')
```
Call | Result
---- | ------
QUTIL::DATEFMT(devicetime) | 2020-07-30


## QUTIL::DATETIMEFMT(timestamp)

*A short function to create a human readable date + time from a timestamp without remembering the correct format string*

This allows to write
```
QUTIL::DATETIMEFMT(devicetime)
```
instead of
```
DATEFORMAT(devicetime, 'yyyy-MM-dd HH:mm:ss')
```
Call | Result
---- | ------
QUTIL::DATETIMEFMT(devicetime) | 2020-07-30 20:54:39


## QUTIL::MAP(value, expression)

*Map value using a JavaScript expression with lodash support*

This function provides access to the lodash JavaScript Library via the included map function. This allows to transform values in AQL queries in a highly flexible way. The expression is the body of a JavaScript function, the given value is available via the variable `e`.

Example: assume we have field containing links (`https://internal.system/analysis?id=123`), but sometimes the `https://` is missing and must be added:
```
QUTIL::MAP(FIELDNAME, '_.startsWith(e, "https://") ? e : "https://" + e')
```


## QUTIL::INTENUMJOIN(separator, items)

*Join an enumeration of integers using separator*

This function allows to join integer enumerations like e.g. `creeventlist` with the given separator.
Enumerations are displayed as "Multiple(n)" in QRadar and you normally cannot see the individual items.

Call | Result
---- | ------
QUTIL::INTENUMJOIN('\|', creeventlist) | `100205|100211|100199`


## QUTIL::STRINGENUMJOIN(separator, items)

*Join an enumeration of strings using separator*

This function allows to join string enumerations like e.g. `RULENAME(creeventlist)` with the given separator.
Enumerations are displayed as "Multiple(n)" in QRadar and you normally cannot see the individual items.

Call | Result
---- | ------
 QUTIL::STRINGENUMJOIN('\|', RULENAME(creeventlist)) | `Destination Asset Weight is Low|Source Asset Weight is Low|Context is Local to Local`

