# Namespace QJSON

Most examples on this page operate on this sample event (as variable `data` in the function calls if not specified otherwise):
```plain
<syslog header> { "key": "value", "array": [ { "key": 1 }, { "key": 2, "dictionary": { "a": "Apple", "b": "Butterfly", "c": "Cat", "d": "Dog" } }, { "key": 3 } ] }
```

In QRadar, the functions can be used with`utf8(payload)` or a Custom Event Property extracting JSON data as string.


As formatted JSON:
```json
{
  "key": "value",
  "array": [
    {
      "key": 1
    },
    {
      "key": 2,
      "dictionary": {
        "a": "Apple",
        "b": "Butterfly",
        "c": "Cat",
        "d": "Dog"
      }
    },
    {
      "key": 3
    }
  ]
}
```

## QJSON::VALUE(data, key)

*Retrieve a value from a JSON object by key*

Call | Result
---- | ------
`QJSON::VALUE(data, 'key');` | `value`


## QJSON::GET(data, path)

*Retrieve a value from a JSON object by path*

This function uses the _get functions from the lodash library ([documentation](https://lodash.com/docs/#get)) to extract a value by path from the json data.

Call | Result
---- | ------
`QJSON::GET(data, 'array[1].dictionary.b')` | `Butterfly`



## QJSON::QUERY(data, expression)

*Retrieve a value from a JSON object using a JSONata expression*

This functions provides advanced filtering and querying capabilities to extract data from JSON using the [JSONata](https://jsonata.org) library.

Call | Result
---- | ------
`QJSON::QUERY(data, 'array[key=2].dictionary.b')` | `Butterfly`
data = `<syslog header> { "example": [ {"value": 4}, {"value": 7}, {"value": 13} ] }` <br /> `QJSON::QUERY(data, '$sum(example.value)');` | `24`
