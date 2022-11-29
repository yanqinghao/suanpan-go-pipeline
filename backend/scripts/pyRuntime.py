import json
import argparse
import pandas


functionSting = '''
def runScript(inputs):
    outputs = []
    %s
    return run(*inputs)
'''

def defaultLoad(x):
    return x

loadMethods = {
    "string": str,
    "int": int,
    "float": float,
    "json": defaultLoad,
    "bool": defaultLoad
}

def defaultDump(x):
    return x

dumpMethods = {
    str: defaultDump,
    int: defaultDump,
    float: defaultDump,
    dict: defaultDump,
    list: defaultDump,
    bool: defaultDump
}

typeMappings = {
    str: "json",
    int: "json",
    float: "json",
    dict: "json",
    list: "json",
    bool: "json"
}

def run(inputs=None, script=""):
    exec(functionSting % script.replace("\n", "\n    "), globals())
    loadedInputs = []
    for input in inputs:
        input = json.loads(eval("'{}'".format(input)))
        loadedInputs.append(loadMethods[input["type"]](input["data"]))
    outputs = runScript(loadedInputs)
    dumpedOutputs = []
    for output in outputs.getAll():
        if type(output) in dumpMethods:
            dumpedOutputs.append({"data": dumpMethods[type(output)](output), "type": typeMappings[type(output)]})
        else:
            raise Exception(f"type of {output} is not supported.")
    return json.dumps(dumpedOutputs)

if __name__=="__main__":
    parser = argparse.ArgumentParser(description='Process some user define function.')
    parser.add_argument('inputs', metavar='{"data": 1, "type": "int"}', type=str, nargs='+',
                    help='an input for the script function')
    parser.add_argument('--script', dest='script', type=str,
                        help='script function process the inputs.')
    args = parser.parse_args()
    print(run(args.inputs, args.script))
