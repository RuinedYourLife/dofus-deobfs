# Dofus Protocol Deobfuscator

This tool is used to deobfuscate the Dofus protocol.

It uses the clear proto files from [dofus-unity-protocol-builder](https://github.com/LuaxY/dofus-unity-protocol-builder)
which we try to map the now obfuscated ones to.

This repo already contains a set of filtered proto files, which you can use for demo purposes.
*It will complain about missing files in the `protos/decompiled` directory and keep running.*

## Usage

Generate the proto files from the Dofus client, and put them in the `protos/decompiled` directory.
*I use Il2CppDumper to dump the client, then use protodec to generate the proto files.*

Run the tool with the `make` command.

Reports will be generated in the `reports` directory.