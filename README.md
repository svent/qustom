# qustom

qustom is a tool to create and maintain Custom AQL functions for IBM QRadar.

This reposity also contains a collection of custom functions that were created using qustom. Every new release of qustom includes a .tar.gz containing the tool and a zip file containing a generated XML bundle. The zip bundle can be uploaded to QRadar via the web interface (Admin Tab -> Extensions Management).

## Advantages of using qustom

QRadar allows users to create their own functions to be used from within the AQL query language.

However, this requires the user to create a JavaScript function, escape it for use within an XML structure (`i<len` => `i&lt;len`), create that structure and provide additional meta data like function parameter types and return types, e.g.:
```xml
<content>
    <custom_function>
        <namespace>utils</namespace>
        <name>add</name>
        <return_type>number</return_type>
        <parameter_types>number number</parameter_types>
        <execute_function_name>execute</execute_function_name>
        <script_engine>javascript</script_engine>
        <script>
            function execute(input1, input2) {
               return input1 + input2;
            }
        </script>
        <username>user1</username>
    </custom_function>
</content>
```
(see https://www.ibm.com/support/knowledgecenter/SS42VS_SHR/com.ibm.appfw.doc/c_appframework_AQLsamples.html)

To test the created function, the XML bundle needs to be zipped, uploaded, installed and then tested within a query.

**qustom** makes this process easier, more reliable and increases maintainability. A configuration for the above function looks like this (qustom uses the TOML format):
```toml
namespace = "util"

[function.add]
description = "Add two numbers"
source = '''
	function add(a, b) {
		return a+b;
	}
'''

[[function.add.tests]]
call = 'add(10, 20)'
expect = '30'
```

This configuration includes a test call for the created function. qustom includes a JavaScript Parser and VM to parse the function and execute the provided test call. The collected data is then used to automatically infer the function parameter types and the return type, making the process easier and less error prone.
It is possible to manually provide the necessary meta data if a test call cannot be provided, or the inferred types need to be overridden.

qustom then generates a bundle from this configuration:

```plain
$ ./qustom generate --bundle bundle.xml add.toml
Generated function util::add(a Number, b Number) => Number
wrote bundle to bundle.xml.
```

## Included custom functions

Some custom functions are included within this repository and a bundle for QRadar is generated for every [release](https://github.com/svent/qustom/releases).
The functions are defined in the [functions](/functions) folder within the repository - each file contains the functions for one "namespace", a way to group functions.

Please see the [documentation](docs/README.md) for more information about the included functions.
