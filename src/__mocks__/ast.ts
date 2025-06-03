export const adHocQueryAST = {
  SelectPos: 0,
  StatementEnd: 57,
  With: null,
  Top: null,
  SelectItems: [
    {
      Expr: { Name: "column1", QuoteType: 1, NamePos: 7, NameEnd: 14 },
      Modifiers: [],
      Alias: null,
    },
    {
      Expr: { Name: "columnt2", QuoteType: 1, NamePos: 16, NameEnd: 24 },
      Modifiers: [],
      Alias: null,
    },
  ],
  From: {
    FromPos: 25,
    Expr: {
      Table: {
        TablePos: 30,
        TableEnd: 35,
        Alias: null,
        Expr: {
          Database: null,
          Table: { Name: "table", QuoteType: 1, NamePos: 30, NameEnd: 35 },
        },
        HasFinal: false,
      },
      StatementEnd: 35,
      SampleRatio: null,
      HasFinal: false,
    },
  },
  ArrayJoin: null,
  Window: null,
  Prewhere: null,
  Where: {
    WherePos: 36,
    Expr: {
      Name: {
        Name: "$__adHocFilter",
        QuoteType: 1,
        NamePos: 42,
        NameEnd: 56,
      },
      Params: {
        LeftParenPos: 56,
        RightParenPos: 57,
        Items: { ListPos: 57, ListEnd: 57, HasDistinct: false, Items: [] },
        ColumnArgList: null,
      },
    },
  },
  GroupBy: null,
  WithTotal: false,
  Having: null,
  OrderBy: null,
  LimitBy: null,
  Limit: null,
  Settings: null,
  Format: null,
  UnionAll: null,
  UnionDistinct: null,
  Except: null,
};
