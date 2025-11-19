.PHONY: clean all

CC ?= cc

ARCH=$(shell uname -m)
LIB_NAME=elastic-jvmti-linux-$(ARCH).so

ifeq ($(ARCH),aarch64)
TLS_DIALECT?=desc
else
TLS_DIALECT?=gnu2
endif

all: fake-apm-agent $(LIB_NAME)

$(LIB_NAME): fake-apm-agent-lib.c
	$(CC) $< -g -shared -fPIC -mtls-dialect=$(TLS_DIALECT) -ftls-model=global-dynamic -o $@

fake-apm-agent: fake-apm-agent.c
	$(CC) $< -g -DLIB_NAME='"$(LIB_NAME)"' -o $@

clean:
	rm elastic-jvmti-linux-$(ARCH).so
	rm fake-apm-agent
