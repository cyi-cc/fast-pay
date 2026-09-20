import { Client } from "./client";
import adminSvc from "./adminSvc";
import appSvc from "./appSvc";
import authSvc from "./authSvc";
import cashierSvc from "./cashierSvc";
import orderSvc from "./orderSvc";
import settleSvc from "./settleSvc";
import upstreamSvc from "./upstreamSvc";
import userSvc from "./userSvc";

export class defaultApi extends Client {
  constructor(url: string) {
    super(url);
  }
  public adminSvc: adminSvc = new adminSvc(this);
  public appSvc: appSvc = new appSvc(this);
  public authSvc: authSvc = new authSvc(this);
  public cashierSvc: cashierSvc = new cashierSvc(this);
  public orderSvc: orderSvc = new orderSvc(this);
  public settleSvc: settleSvc = new settleSvc(this);
  public upstreamSvc: upstreamSvc = new upstreamSvc(this);
  public userSvc: userSvc = new userSvc(this);
}

export default class api {
  static create(url: string): defaultApi {
    return new defaultApi(url);
  }
}