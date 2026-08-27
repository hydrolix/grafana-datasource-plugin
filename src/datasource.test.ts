import {firstValueFrom, of} from "rxjs";
import {CoreApp, DataQueryRequest, dateTime, toDataFrame} from "@grafana/data";
import {MockDataSourceInstanceSettings, setupDataSourceMock,} from "__mocks__/datasource";
import {adHocTableVariable, fooVariable} from "./__mocks__/variable";
import {AdHocFilterKeys, HdxQuery} from "./types";
import {ZERO_TIME_RANGE} from "./editor/metadataProvider";
import {AD_HOC_PRELOAD_LOOKBACK_SECONDS} from "./constants";

describe("HdxDataSource", () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    describe("When performing metricFindQuery", () => {
        beforeEach(() => {
            jest.clearAllMocks();
        });

        const cases: Array<{
            name: string;
            response: any;
            expected: any;
        }> = [
            {
                name: "it should return values",
                response: {
                    fields: [{name: "values", type: "number", values: [100, 200]}],
                },
                expected: [
                    {text: 100, value: 100},
                    {text: 200, value: 200},
                ],
            },
            {
                name: "it should return identified values",
                response: {
                    fields: [
                        {name: "ids", type: "number", values: [1, 2]},
                        {name: "values", type: "number", values: [100, 200]},
                    ],
                },
                expected: [
                    {text: 100, value: 1},
                    {text: 200, value: 2},
                ],
            },
        ];

        const {datasource, queryMock} = setupDataSourceMock({});

        test.each(cases)("$name", async ({response, expected}) => {
            queryMock.mockImplementation((_) =>
                of({data: [toDataFrame(response)]})
            );
            const actual = await datasource.metricFindQuery("mock", {});
            expect(actual).toEqual(expected);
        });
    });

    const filterQueryCases: Array<{ query: string; valid: boolean }> = [
        {query: "", valid: false},
        {query: "select 1;", valid: true},
    ];

    test.each(filterQueryCases)(
        "should filter out invalid query",
        ({query, valid}) => {
            const {datasource} = setupDataSourceMock({});
            const actual = datasource.filterQuery({
                refId: "",
                rawSql: query,
                round: "",
                querySettings: [],
            });
            expect(actual).toEqual(valid);
        }
    );

    it("should interpolate variables in the query", async () => {
        const {datasource} = setupDataSourceMock({
            variables: [fooVariable],
        });
        const actual = datasource.applyTemplateVariables(
            {
                refId: "",
                rawSql: "foo $foo",
                round: "",
                querySettings: [],
            },
            {}
        );
        expect(actual.rawSql).toEqual("foo templatedFoo");
    });

    describe("ad hoc filtering", () => {
        beforeEach(() => {
            jest.clearAllMocks();
        });
        const {datasource, queryMock} = setupDataSourceMock({
            variables: [adHocTableVariable],
        });
        const getKeysMock = jest.spyOn(datasource.metadataProvider, "tableKeys");

        it("should return keys", async () => {
            let response = ["key1", "key2", "key3"].map(
                (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
            );
            getKeysMock.mockReturnValue(Promise.resolve(response));
            queryMock.mockReturnValue(of({data: []}));
            let keys = await datasource.getTagKeys();

            expect(keys).toEqual(response);
        });

        it("should not return values", async () => {
            let response = ["key1", "key2", "key3"].map(
                (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
            );
            getKeysMock.mockReturnValue(Promise.resolve(response));
            let values = await datasource.getTagValues({key: "key", filters: []});

            expect(values).toEqual([]);
        });

        it("should  return values", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve(
                    ["key1", "key2", "key3"].map(
                        (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
                    )
                )
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: [100, 200]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([100, 200].map((k) => ({text: k, value: k})));
        });

        it("should append synthetic null for a Nullable column type even without null rows", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "Nullable(String)"},
                    {text: "key2", value: "key2", type: "String"},
                    {text: "key3", value: "key3", type: "String"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["a", "b"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([
                {text: "a", value: "a"},
                {text: "b", value: "b"},
                {text: "__null__", value: "__null__"},
            ]);
        });

        it("should not append synthetic null for a non-Nullable column type", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve(
                    ["key1", "key2", "key3"].map(
                        (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
                    )
                )
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["a", "b"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([
                {text: "a", value: "a"},
                {text: "b", value: "b"},
            ]);
        });

        it("should drop raw null entries from a non-Nullable column's value list", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve(
                    ["key1", "key2", "key3"].map(
                        (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
                    )
                )
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: [null]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([]);
        });

        it("should return an empty list without error for an empty successful response", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve(
                    ["key1", "key2", "key3"].map(
                        (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
                    )
                )
            );
            queryMock.mockReturnValue(of({data: []}));
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([]);
        });

        it("should return empty value", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve(
                    ["key1", "key2", "key3"].map(
                        (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
                    )
                )
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: [""]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([{text: "__empty__", value: "__empty__"}]);
        });

        it("should return empty and synthetic value", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve(
                    ["key1", "key2", "key3"].map(
                        (k) => ({text: k, value: k, type: "String"} as AdHocFilterKeys)
                    )
                )
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["", "__empty__"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([{text: "__empty__", value: "__empty__"}]);
        });
        it("should dedupe a raw '__null__' value against the type-gated synthetic null", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "Nullable(String)"},
                    {text: "key2", value: "key2", type: "String"},
                    {text: "key3", value: "key3", type: "String"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: [null, "__null__"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([{text: "__null__", value: "__null__"}]);
        });
        it("should return empty, null and both synthetic values for a Nullable column", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "Nullable(String)"},
                    {text: "key2", value: "key2", type: "String"},
                    {text: "key3", value: "key3", type: "String"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: [null, "__null__", "", "__empty__"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([
                {text: "__empty__", value: "__empty__"},
                {text: "__null__", value: "__null__"},
            ]);
        });

        it("should use arrayJoin for array type columns", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "Array(String)"},
                    {text: "key2", value: "key2", type: "String"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["tag1", "tag2", "tag3"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([
                {text: "tag1", value: "tag1"},
                {text: "tag2", value: "tag2"},
                {text: "tag3", value: "tag3"},
            ]);
        });

        it("should not use arrayJoin for non-array type columns", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "String"},
                    {text: "key2", value: "key2", type: "Nullable(String)"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["value1", "value2"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "key1", filters: []});

            expect(values).toEqual([
                {text: "value1", value: "value1"},
                {text: "value2", value: "value2"},
            ]);
        });

        it("should handle Array(Nullable(String)) type", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "tags", value: "tags", type: "Array(Nullable(String))"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["prod", "staging", "dev"]}],
                        }),
                    ],
                })
            );
            let values = await datasource.getTagValues({key: "tags", filters: []});

            expect(values).toEqual([
                {text: "prod", value: "prod"},
                {text: "staging", value: "staging"},
                {text: "dev", value: "dev"},
            ]);
        });

        it("should expand map type columns into individual keys", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "column1", value: "column1", type: "String"},
                    {text: "labels", value: "labels", type: "Map(String, String)"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["env", "region", "team"]}],
                        }),
                    ],
                })
            );

            let keys = await datasource.getTagKeys();

            expect(keys).toContainEqual({
                text: "column1",
                value: "column1",
                type: "String",
            });
            expect(keys).toContainEqual({
                text: "labels['env']",
                value: "labels['env']",
                type: "Map(String, String)",
            });
            expect(keys).toContainEqual({
                text: "labels['region']",
                value: "labels['region']",
                type: "Map(String, String)",
            });
            expect(keys).toContainEqual({
                text: "labels['team']",
                value: "labels['team']",
                type: "Map(String, String)",
            });
        });

        it("should handle map key syntax in getTagValues", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "labels", value: "labels", type: "Map(String, String)"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["prod", "dev", "staging"]}],
                        }),
                    ],
                })
            );

            let values = await datasource.getTagValues({
                key: "labels['env']",
                filters: [],
            });

            expect(values).toEqual([
                {text: "prod", value: "prod"},
                {text: "dev", value: "dev"},
                {text: "staging", value: "staging"},
            ]);
        });

        it("should handle nullable map type", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {
                        text: "metadata",
                        value: "metadata",
                        type: "Map(String, Nullable(String))",
                    },
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["key1", "key2"]}],
                        }),
                    ],
                })
            );

            let keys = await datasource.getTagKeys();

            expect(keys).toContainEqual({
                text: "metadata['key1']",
                value: "metadata['key1']",
                type: "Map(String, Nullable(String))",
            });
            expect(keys).toContainEqual({
                text: "metadata['key2']",
                value: "metadata['key2']",
                type: "Map(String, Nullable(String))",
            });
        });

        it("should append synthetic null for a map key on a Map(String, Nullable(String)) column", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {
                        text: "metadata",
                        value: "metadata",
                        type: "Map(String, Nullable(String))",
                    },
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["prod", "dev"]}],
                        }),
                    ],
                })
            );

            let values = await datasource.getTagValues({
                key: "metadata['env']",
                filters: [],
            });

            expect(values).toEqual([
                {text: "prod", value: "prod"},
                {text: "dev", value: "dev"},
                {text: "__null__", value: "__null__"},
            ]);
        });

        it("should not append synthetic null for a map key on a Map(String, String) column", async () => {
            getKeysMock.mockReturnValue(
                Promise.resolve([
                    {text: "labels", value: "labels", type: "Map(String, String)"},
                ] as AdHocFilterKeys[])
            );
            queryMock.mockReturnValue(
                of({
                    data: [
                        toDataFrame({
                            fields: [{values: ["prod", "dev"]}],
                        }),
                    ],
                })
            );

            let values = await datasource.getTagValues({
                key: "labels['env']",
                filters: [],
            });

            expect(values).toEqual([
                {text: "prod", value: "prod"},
                {text: "dev", value: "dev"},
            ]);
        });
    });

    it("should process error", async () => {
        const {datasource, queryMock} = setupDataSourceMock({});
        queryMock.mockReturnValue(
            of({data: [], errors: [{message: "error message", status: "error"}]})
        );
        const req = {
            targets: [{rawSql: "select 1", refId: String(Math.random())}],
        } as DataQueryRequest<HdxQuery>;
        let a = await firstValueFrom(datasource.query(req));
        expect(a.errors![0].message).toBe("error message");
    });

    describe("query settings", () => {
        it("should merge datasource-level and query-level settings", async () => {
            const {datasource, queryMock} = setupDataSourceMock({
                customInstanceSettings: {
                    ...MockDataSourceInstanceSettings,
                    jsonData: {
                        ...MockDataSourceInstanceSettings.jsonData,
                        querySettings: [{setting: "hdx_query_max_rows", value: "1000"}],
                    },
                },
            });
            queryMock.mockReturnValue(of({data: []}));
            const req = {
                targets: [
                    {
                        rawSql: "select 1",
                        refId: "A",
                        querySettings: [{setting: "hdx_query_max_attempts", value: "5"}],
                    },
                ],
            } as DataQueryRequest<HdxQuery>;
            await firstValueFrom(datasource.query(req));
            const sentTarget = queryMock.mock.calls[0][0].targets[0];
            expect(sentTarget.querySettings).toEqual(
                expect.arrayContaining([
                    expect.objectContaining({
                        setting: "hdx_query_max_rows",
                        value: "1000",
                    }),
                    expect.objectContaining({
                        setting: "hdx_query_max_attempts",
                        value: "5",
                    }),
                ])
            );
        });

        it("should allow query-level settings to override datasource-level", async () => {
            const {datasource, queryMock} = setupDataSourceMock({
                customInstanceSettings: {
                    ...MockDataSourceInstanceSettings,
                    jsonData: {
                        ...MockDataSourceInstanceSettings.jsonData,
                        querySettings: [{setting: "hdx_query_max_rows", value: "1000"}],
                    },
                },
            });
            queryMock.mockReturnValue(of({data: []}));
            const req = {
                targets: [
                    {
                        rawSql: "select 1",
                        refId: "A",
                        querySettings: [{setting: "hdx_query_max_rows", value: "500"}],
                    },
                ],
            } as DataQueryRequest<HdxQuery>;
            await firstValueFrom(datasource.query(req));
            const sentTarget = queryMock.mock.calls[0][0].targets[0];
            const maxRows = sentTarget.querySettings.find(
                (s: any) => s.setting === "hdx_query_max_rows"
            );
            expect(maxRows.value).toBe("500");
        });

        it("should filter out settings with empty setting name", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));
            const req = {
                targets: [
                    {
                        rawSql: "select 1",
                        refId: "A",
                        querySettings: [
                            {setting: "", value: "ignored"},
                            {setting: "hdx_query_max_rows", value: "100"},
                        ],
                    },
                ],
            } as DataQueryRequest<HdxQuery>;
            await firstValueFrom(datasource.query(req));
            const sentTarget = queryMock.mock.calls[0][0].targets[0];
            expect(sentTarget.querySettings).toEqual([
                expect.objectContaining({
                    setting: "hdx_query_max_rows",
                    value: "100",
                }),
            ]);
            expect(
                sentTarget.querySettings.find((s: any) => s.setting === "")
            ).toBeUndefined();
        });

        it("should handle empty querySettings arrays", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));
            const req = {
                targets: [
                    {
                        rawSql: "select 1",
                        refId: "A",
                        querySettings: [],
                    },
                ],
            } as unknown as DataQueryRequest<HdxQuery>;
            await firstValueFrom(datasource.query(req));
            const sentTarget = queryMock.mock.calls[0][0].targets[0];
            expect(sentTarget.querySettings).toEqual([]);
        });

        it("should handle undefined querySettings on target", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));
            const req = {
                targets: [
                    {
                        rawSql: "select 1",
                        refId: "A",
                    },
                ],
            } as DataQueryRequest<HdxQuery>;
            await firstValueFrom(datasource.query(req));
            const sentTarget = queryMock.mock.calls[0][0].targets[0];
            expect(sentTarget.querySettings).toEqual([]);
        });

        it("should replace template variables in setting values", async () => {
            const {datasource, queryMock} = setupDataSourceMock({
                variables: [fooVariable],
            });
            queryMock.mockReturnValue(of({data: []}));
            const req = {
                targets: [
                    {
                        rawSql: "select 1",
                        refId: "A",
                        querySettings: [
                            {setting: "hdx_query_admin_comment", value: "$foo"},
                        ],
                    },
                ],
            } as DataQueryRequest<HdxQuery>;
            await firstValueFrom(datasource.query(req));
            const sentTarget = queryMock.mock.calls[0][0].targets[0];
            const comment = sentTarget.querySettings.find(
                (s: any) => s.setting === "hdx_query_admin_comment"
            );
            expect(comment.value).toBe("templatedFoo");
        });

        describe("hydrolix-namespaced synthetic variables", () => {
            it("expands ${__hydrolix.panel.id}, .panel.name, .app, .ref_id in a dashboard context", async () => {
                const {datasource, queryMock} = setupDataSourceMock({});
                queryMock.mockReturnValue(of({data: []}));
                const req = {
                    app: "dashboard",
                    panelId: 12,
                    panelName: "Throughput",
                    targets: [
                        {
                            rawSql: "select 1",
                            refId: "A",
                            querySettings: [
                                {
                                    setting: "hdx_query_admin_comment",
                                    value:
                                        "p=${__hydrolix.panel.id};n=${__hydrolix.panel.name};a=${__hydrolix.app};r=${__hydrolix.ref_id}",
                                },
                            ],
                        },
                    ],
                } as unknown as DataQueryRequest<HdxQuery>;
                await firstValueFrom(datasource.query(req));
                const sentTarget = queryMock.mock.calls[0][0].targets[0];
                const comment = sentTarget.querySettings.find(
                    (s: any) => s.setting === "hdx_query_admin_comment"
                );
                expect(comment.value).toBe("p=12;n=Throughput;a=dashboard;r=A");
            });

            it("expands missing panelId / panelName to empty strings (Explore-like context)", async () => {
                const {datasource, queryMock} = setupDataSourceMock({});
                queryMock.mockReturnValue(of({data: []}));
                const req = {
                    app: "explore",
                    // panelId and panelName intentionally undefined
                    targets: [
                        {
                            rawSql: "select 1",
                            refId: "A",
                            querySettings: [
                                {
                                    setting: "hdx_query_admin_comment",
                                    value:
                                        "p=${__hydrolix.panel.id};n=${__hydrolix.panel.name};a=${__hydrolix.app}",
                                },
                            ],
                        },
                    ],
                } as unknown as DataQueryRequest<HdxQuery>;
                await firstValueFrom(datasource.query(req));
                const sentTarget = queryMock.mock.calls[0][0].targets[0];
                const comment = sentTarget.querySettings.find(
                    (s: any) => s.setting === "hdx_query_admin_comment"
                );
                expect(comment.value).toBe("p=;n=;a=explore");
            });

            it("reflects annotation context via __hydrolix.app and __hydrolix.ref_id", async () => {
                const {datasource, queryMock} = setupDataSourceMock({});
                queryMock.mockReturnValue(of({data: []}));
                const req = {
                    app: "annotation",
                    targets: [
                        {
                            rawSql: "select 1",
                            refId: "Anno",
                            querySettings: [
                                {
                                    setting: "hdx_query_admin_comment",
                                    value: "a=${__hydrolix.app};r=${__hydrolix.ref_id}",
                                },
                            ],
                        },
                    ],
                } as unknown as DataQueryRequest<HdxQuery>;
                await firstValueFrom(datasource.query(req));
                const sentTarget = queryMock.mock.calls[0][0].targets[0];
                const comment = sentTarget.querySettings.find(
                    (s: any) => s.setting === "hdx_query_admin_comment"
                );
                expect(comment.value).toBe("a=annotation;r=Anno");
            });

            it("treats panelId=0 as a present id (not as the empty-fallback)", async () => {
                const {datasource, queryMock} = setupDataSourceMock({});
                queryMock.mockReturnValue(of({data: []}));
                const req = {
                    app: "dashboard",
                    panelId: 0,
                    panelName: "",
                    targets: [
                        {
                            rawSql: "select 1",
                            refId: "A",
                            querySettings: [
                                {
                                    setting: "hdx_query_admin_comment",
                                    value: "p=${__hydrolix.panel.id};n=${__hydrolix.panel.name}",
                                },
                            ],
                        },
                    ],
                } as unknown as DataQueryRequest<HdxQuery>;
                await firstValueFrom(datasource.query(req));
                const sentTarget = queryMock.mock.calls[0][0].targets[0];
                const comment = sentTarget.querySettings.find(
                    (s: any) => s.setting === "hdx_query_admin_comment"
                );
                expect(comment.value).toBe("p=0;n=");
            });
        });
    });

    describe("annotation request retag", () => {
        beforeEach(() => {
            jest.clearAllMocks();
        });

        const realRange = {
            from: {valueOf: () => 1} as any,
            to: {valueOf: () => 2} as any,
            raw: {from: "now-1h", to: "now"},
        } as any;

        function buildRequest(
            targets: Array<Partial<HdxQuery>>,
            overrides: Partial<DataQueryRequest<HdxQuery>> = {}
        ): DataQueryRequest<HdxQuery> {
            return {
                app: CoreApp.Dashboard,
                range: realRange,
                targets: targets.map(
                    (t, i) =>
                        ({
                            refId: `R${i}`,
                            rawSql: "SELECT 1",
                            round: "",
                            querySettings: [],
                            ...t,
                        } as HdxQuery)
                ),
                filters: [],
                ...overrides,
            } as DataQueryRequest<HdxQuery>;
        }

        it("retags annotation requests to app='annotation' before super.query", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));

            const req = buildRequest([{source: "annotation"}]);
            await firstValueFrom(datasource.query(req));

            expect(queryMock.mock.calls[0][0].app).toBe("annotation");
        });

        it("does not retag panel requests", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));

            const req = buildRequest([{}]);
            await firstValueFrom(datasource.query(req));

            expect(queryMock.mock.calls[0][0].app).toBe(CoreApp.Dashboard);
        });

        it("does not overwrite this.filters from an annotation request", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));

            const panelFilters = [{key: "k", operator: "=", value: "v"}];
            await firstValueFrom(
                datasource.query(buildRequest([{}], {filters: panelFilters as any}))
            );
            expect(datasource.filters).toEqual(panelFilters);

            await firstValueFrom(
                datasource.query(
                    buildRequest([{source: "annotation"}], {filters: []})
                )
            );
            expect(datasource.filters).toEqual(panelFilters);
        });

        it("updates this.filters for a panel request (regression check)", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));

            const panelFilters = [{key: "k2", operator: "=", value: "v2"}];
            await firstValueFrom(
                datasource.query(buildRequest([{}], {filters: panelFilters as any}))
            );
            expect(datasource.filters).toEqual(panelFilters);
        });

        it("updates this.options for an annotation request with a real range", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));

            const req = buildRequest([{source: "annotation"}]);
            await firstValueFrom(datasource.query(req));

            expect(datasource.options?.range).toBe(realRange);
            expect(datasource.options?.app).toBe("annotation");
        });

        it("leaves this.options unchanged for ZERO_TIME_RANGE", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));

            const prior = buildRequest([{}]);
            await firstValueFrom(datasource.query(prior));
            const snapshot = datasource.options;

            const zero = buildRequest([{source: "annotation"}], {
                range: ZERO_TIME_RANGE as any,
            });
            await firstValueFrom(datasource.query(zero));
            expect(datasource.options).toBe(snapshot);
        });

        it("does not mutate the original DataQueryRequest", async () => {
            const {datasource, queryMock} = setupDataSourceMock({});
            queryMock.mockReturnValue(of({data: []}));

            const req = buildRequest([{source: "annotation"}]);
            const originalApp = req.app;
            const originalTargets = req.targets;

            await firstValueFrom(datasource.query(req));

            expect(req.app).toBe(originalApp);
            expect(req.targets).toBe(originalTargets);
        });
    });

    describe("preload time range capping", () => {
        beforeEach(() => {
            jest.clearAllMocks();
        });

        function makeRange(fromMs: number, toMs: number) {
            const from = dateTime(fromMs);
            const to = dateTime(toMs);
            return {from, to, raw: {from, to}};
        }

        it("caps a 90-day range to the trailing 24h for getTagValues", async () => {
            const {datasource, queryMock} = setupDataSourceMock({
                variables: [adHocTableVariable],
            });
            jest.spyOn(datasource.metadataProvider, "tableKeys").mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "String"},
                ] as AdHocFilterKeys[])
            );
            jest
                .spyOn(datasource.metadataProvider, "primaryKey")
                .mockReturnValue(Promise.resolve("ts"));
            queryMock.mockReturnValue(of({data: []}));

            const to = 1_700_000_000_000;
            const from = to - 90 * 24 * 60 * 60 * 1000;
            await datasource.getTagValues({
                key: "key1",
                filters: [],
                timeRange: makeRange(from, to) as any,
            });

            const sentRange = queryMock.mock.calls[0][0].range;
            expect(sentRange.from.valueOf()).toBe(
                to - AD_HOC_PRELOAD_LOOKBACK_SECONDS * 1000
            );
            expect(sentRange.to.valueOf()).toBe(to);
        });

        it("leaves a 6-hour range untouched for getTagValues", async () => {
            const {datasource, queryMock} = setupDataSourceMock({
                variables: [adHocTableVariable],
            });
            jest.spyOn(datasource.metadataProvider, "tableKeys").mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "String"},
                ] as AdHocFilterKeys[])
            );
            jest
                .spyOn(datasource.metadataProvider, "primaryKey")
                .mockReturnValue(Promise.resolve("ts"));
            queryMock.mockReturnValue(of({data: []}));

            const to = 1_700_000_000_000;
            const from = to - 6 * 60 * 60 * 1000;
            await datasource.getTagValues({
                key: "key1",
                filters: [],
                timeRange: makeRange(from, to) as any,
            });

            const sentRange = queryMock.mock.calls[0][0].range;
            expect(sentRange.from.valueOf()).toBe(from);
            expect(sentRange.to.valueOf()).toBe(to);
        });

        it("caps this.options.range for getTagKeysForMap on a long dashboard range", async () => {
            const {datasource, queryMock} = setupDataSourceMock({
                variables: [adHocTableVariable],
            });
            queryMock.mockReturnValue(of({data: []}));

            const to = 1_700_000_000_000;
            const from = to - 90 * 24 * 60 * 60 * 1000;
            const req = {
                app: CoreApp.Dashboard,
                range: makeRange(from, to),
                targets: [
                    {
                        refId: "A",
                        rawSql: "SELECT 1",
                        round: "",
                        querySettings: [],
                    },
                ],
                filters: [],
            } as unknown as DataQueryRequest<HdxQuery>;
            await firstValueFrom(datasource.query(req));
            queryMock.mockClear();

            await datasource.getTagKeysForMap("labels", "sample.table");

            const sentRange = queryMock.mock.calls[0][0].range;
            expect(sentRange.from.valueOf()).toBe(
                to - AD_HOC_PRELOAD_LOOKBACK_SECONDS * 1000
            );
            expect(sentRange.to.valueOf()).toBe(to);
        });


        it("caps range.raw.from alongside range.from for getTagValues", async () => {
            const {datasource, queryMock} = setupDataSourceMock({
                variables: [adHocTableVariable],
            });
            jest.spyOn(datasource.metadataProvider, "tableKeys").mockReturnValue(
                Promise.resolve([
                    {text: "key1", value: "key1", type: "String"},
                ] as AdHocFilterKeys[])
            );
            jest
                .spyOn(datasource.metadataProvider, "primaryKey")
                .mockReturnValue(Promise.resolve("ts"));
            queryMock.mockReturnValue(of({data: []}));

            const to = 1_700_000_000_000;
            const from = to - 90 * 24 * 60 * 60 * 1000;
            await datasource.getTagValues({
                key: "key1",
                filters: [],
                timeRange: makeRange(from, to) as any,
            });

            // `raw` is the unresolved form of the same window - leaving it at the
            // uncapped bound makes the returned TimeRange self-contradictory.
            const sentRange = queryMock.mock.calls[0][0].range;
            expect(sentRange.raw.from.valueOf()).toBe(
                to - AD_HOC_PRELOAD_LOOKBACK_SECONDS * 1000
            );
            expect(sentRange.raw.to.valueOf()).toBe(to);
        });
    });
});
