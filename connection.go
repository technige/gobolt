/*
 * Copyright (c) 2002-2018 "Neo4j,"
 * Neo4j Sweden AB [http://neo4j.com]
 *
 * This file is part of Neo4j.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package neo4j_go_connector

/*
#cgo pkg-config: seabolt

#include <stdlib.h>

#include "bolt/connections.h"
*/
import "C"
import (
    "errors"
    "unsafe"
)

type Connection interface {
    Begin(bookmarks []string) (RequestHandle, error)
    Commit() (RequestHandle, error)
    Rollback() (RequestHandle, error)
    Run(cypher string, args *map[string]interface{}) (RequestHandle, error)
    PullAll() (RequestHandle, error)
    DiscardAll() (RequestHandle, error)
    Flush() error
    Fetch(request RequestHandle) (FetchType, error)  // return type ?
    FetchSummary(request RequestHandle) (int, error) // return type ?

    LastBookmark() (string)
    Metadata() (map[string]interface{}, error)
    Data() ([]interface{}, error)

    Reset() error
    Close() error
}

type neo4jConnection struct {
    pool      *neo4jPool
    cInstance *C.struct_BoltConnection
}

func (connection *neo4jConnection) Begin(bookmarks []string) (RequestHandle, error) {
    for _, bookmark := range bookmarks {
        bookmarkString := C.CString(bookmark)
        defer C.free(unsafe.Pointer(bookmarkString))
        res := C.BoltConnection_load_bookmark(connection.cInstance, bookmarkString)
        if res < 0 {
            return -1, errors.New("unable to load bookmark")
        }
    }

    res := C.BoltConnection_load_begin_request(connection.cInstance)
    if res < 0 {
        return -1, errors.New("unable to generate BEGIN message")
    }

    return RequestHandle(C.BoltConnection_last_request(connection.cInstance)), nil
}

func (connection *neo4jConnection) Commit() (RequestHandle, error) {
    res := C.BoltConnection_load_commit_request(connection.cInstance)
    if res < 0 {
        return -1, errors.New("unable to generate COMMIT message")
    }

    return RequestHandle(C.BoltConnection_last_request(connection.cInstance)), nil
}

func (connection *neo4jConnection) Rollback() (RequestHandle, error) {
    res := C.BoltConnection_load_rollback_request(connection.cInstance)
    if res < 0 {
        return -1, errors.New("unable to generate ROLLBACK message")
    }

    return RequestHandle(C.BoltConnection_last_request(connection.cInstance)), nil
}

func (connection *neo4jConnection) Run(cypher string, params *map[string]interface{}) (RequestHandle, error) {
    stmt := C.CString(cypher)
    defer C.free(unsafe.Pointer(stmt))

    var actualParams map[string]interface{}
    if params == nil {
        actualParams = map[string]interface{}(nil)
    } else {
        actualParams = *params
    }

    res := C.BoltConnection_cypher(connection.cInstance, stmt, C.size_t(len(cypher)), C.int32_t(len(actualParams)))
    if res < 0 {
        return -1, errors.New("unable to set cypher statement")
    }

    i := 0
    for k, v := range actualParams {
        index := C.int32_t(i)
        key := C.CString(k)

        boltValue := C.BoltConnection_cypher_parameter(connection.cInstance, index, key, C.size_t(len(k)))
        if boltValue == nil {
            return -1, errors.New("unable to get cypher statement parameter value to set")
        }

        valueAsConnector(boltValue, v)

        i += 1
    }

    res = C.BoltConnection_load_run_request(connection.cInstance)
    if res < 0 {
        return -1, errors.New("unable to generate RUN message")
    }

    return RequestHandle(C.BoltConnection_last_request(connection.cInstance)), nil
}

func (connection *neo4jConnection) PullAll() (RequestHandle, error) {
    res := C.BoltConnection_load_pull_request(connection.cInstance, -1)
    if res < 0 {
        return -1, errors.New("unable to generate PULLALL message")
    }
    return RequestHandle(C.BoltConnection_last_request(connection.cInstance)), nil
}

func (connection *neo4jConnection) DiscardAll() (RequestHandle, error) {
    res := C.BoltConnection_load_discard_request(connection.cInstance, -1)
    if res < 0 {
        return -1, errors.New("unable to generate DISCARDALL message")
    }
    return RequestHandle(C.BoltConnection_last_request(connection.cInstance)), nil
}

func (connection *neo4jConnection) assertReadyState() error {
    if connection.cInstance.status != C.BOLT_READY {
        if connection.cInstance.error == C.BOLT_SERVER_FAILURE {
            status := valueAsDictionary(C.BoltConnection_failure(connection.cInstance))

            return newDatabaseError(status)
        }

        return newConnectionError(connection)
    }

    return nil
}

func (connection *neo4jConnection) Flush() error {
    res := C.BoltConnection_send(connection.cInstance)
    if res < 0 {
        return errors.New("unable to send pending messages")
    }

    return connection.assertReadyState()
}

func (connection *neo4jConnection) Fetch(request RequestHandle) (FetchType, error) {
    res := C.BoltConnection_fetch(connection.cInstance, C.bolt_request_t(request))

    if err := connection.assertReadyState(); err != nil {
        return ERROR, err
    }

    return FetchType(res), nil
}

func (connection *neo4jConnection) FetchSummary(request RequestHandle) (int, error) {
    res := C.BoltConnection_fetch_summary(connection.cInstance, C.bolt_request_t(request))
    if res < 0 {
        return -1, errors.New("unable to fetch summary from connection")
    }

    err := connection.assertReadyState()
    if err != nil {
        return -1, err
    }

    return int(res), nil
}

func (connection *neo4jConnection) LastBookmark() string {
    bookmark := C.BoltConnection_last_bookmark(connection.cInstance)
    if bookmark != nil {
        return C.GoString(bookmark)
    }
    return ""
}

func (connection *neo4jConnection) Metadata() (map[string]interface{}, error) {
    metadata := make(map[string]interface{}, 1)

    fields, err := valueAsGo(C.BoltConnection_metadata_fields(connection.cInstance))
    if err != nil {
        return nil, err
    }

    if fields != nil {
        fieldsAsList := fields.([]interface{})
        fieldsAsStr := make([]string, len(fieldsAsList))
        for i := range fieldsAsList {
            fieldsAsStr[i] = fieldsAsList[i].(string)
        }
        metadata["fields"] = fieldsAsStr
    }

    return metadata, nil
}

func (connection *neo4jConnection) Data() ([]interface{}, error) {
    fields, err := valueAsGo(C.BoltConnection_record_fields(connection.cInstance))
    if err != nil {
        return nil, err
    }

    return fields.([]interface{}), nil
}

func (connection *neo4jConnection) Reset() error {
    res := C.BoltConnection_reset(connection.cInstance)
    if res < 0 {
        return errors.New("unable to reset connection")
    }

    err := connection.assertReadyState()
    if err != nil {
        return err
    }

    return nil
}

func (connection *neo4jConnection) Close() error {
    err := connection.pool.release(connection)
    if err != nil {
        return err
    }
    return nil
}