# PartiQL Explorer

curl --location -O https://github.com/partiql/partiql-lang-kotlin/releases/download/v0.2.4-alpha/partiql-cli-0.2.4.tgz

java -cp cli-0.2.4.jar:jopt-simple-6.0-alpha-3.jar:ion-java-1.7.1.jar:lang-0.2.4.jar:kotlin-stdlib-1.3.72.jar:partiql-ir-generator-runtime-0.3.0.jar:ion-element-0.2.0.jar org.partiql.cli.Main -e ../Tutorial/code/tutorial-all-data.env

