# configuration variables
BUILD_DIR:=build
PARTIQL_VERSION:=0.2.4
JAVA_SRC_DIR:=java
OUTPUT_JAR:=$(BUILD_DIR)/partiqldemo.jar

# variables computed from configuration
PARTIQL_JAR:=$(BUILD_DIR)/partiql-combined-$(PARTIQL_VERSION).jar
JAVA_SRC_FILES:=$(shell find $(JAVA_SRC_DIR) -type f -name '*.java')
JAVA_BUILD_DIR:=$(BUILD_DIR)/$(JAVA_SRC_DIR)
JAVA_BUILD_STAMP:=$(JAVA_BUILD_DIR)/.stamp

all: build_binaries check
build_binaries: $(OUTPUT_JAR) $(BUILD_DIR)/partiqldemo
check:
	goimports -l -w .
	go test -race ./...
	go vet ./...
	golint ./...
	staticcheck ./...
	go mod tidy

$(BUILD_DIR)/partiqldemo:
	go build -o $@ partiqldemo.go

docker: $(OUTPUT_JAR)
	docker build . --tag=gcr.io/gosignin-demo/partiqldemo:$(shell date +%Y%m%d)-$(shell git rev-parse --short=10 HEAD)

$(PARTIQL_JAR): buildtools/makepartiqljar.go | $(BUILD_DIR)
	go run $< --version=$(PARTIQL_VERSION) --outputPath=$@

# add the compiled classes to the combined jar
$(OUTPUT_JAR): $(PARTIQL_JAR) $(JAVA_BUILD_STAMP)
	cp $(PARTIQL_JAR) $(JAVA_BUILD_DIR)/buildtemp.jar
	cd $(JAVA_BUILD_DIR) && find . -type f -name '*.class' | xargs jar --update --main-class=ca.evanjones.partiql.ExecuteQuery --file=buildtemp.jar
	mv $(JAVA_BUILD_DIR)/buildtemp.jar $@

$(JAVA_BUILD_STAMP): $(JAVA_SRC_FILES) $(PARTIQL_JAR) | $(BUILD_DIR)
	mkdir -p $(dir $@)
	javac -Xlint:all -cp $(PARTIQL_JAR) \
		--source-path $(JAVA_SRC_DIR) \
		-d $(JAVA_BUILD_DIR) \
		$(filter %.java,$<)
	touch $@

$(PROTOC_GEN_GO): | $(PROTOC_DIR)
	go build --mod=readonly -o $@ github.com/golang/protobuf/protoc-gen-go

$(BUILD_DIR): $(JAVA_SRC_FILES)
	mkdir -p $@
