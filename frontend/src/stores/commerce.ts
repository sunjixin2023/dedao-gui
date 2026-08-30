import { defineStore } from "pinia";

export type RedirectOrderStatus = "redirected" | "visited" | "archived";

export type RedirectOrder = {
  localId: string;
  title: string;
  cover: string;
  productKind: string;
  productType: number;
  productId: string;
  priceText: string;
  officialUrl: string;
  createdAt: number;
  status: RedirectOrderStatus;
};

type RedirectOrderPayload = {
  title: string;
  cover?: string;
  productKind?: string;
  productType?: number;
  productId?: string;
  priceText?: string;
  officialUrl: string;
};

export const commerceStore = defineStore("commerce", {
  state: () => {
    return {
      redirectOrders: [] as RedirectOrder[],
    };
  },
  getters: {
    orderCount: (state) => state.redirectOrders.length,
    activeOrders: (state) =>
      state.redirectOrders.filter((item) => item.status !== "archived"),
  },
  actions: {
    recordRedirectOrder(payload: RedirectOrderPayload) {
      const title = String(payload.title || "").trim();
      const officialUrl = String(payload.officialUrl || "").trim();
      if (!title || !officialUrl) return null;

      const order: RedirectOrder = {
        localId: `redirect_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
        title,
        cover: String(payload.cover || "").trim(),
        productKind: String(payload.productKind || "content").trim(),
        productType: Number(payload.productType || 0),
        productId: String(payload.productId || "").trim(),
        priceText: String(payload.priceText || "官网下单").trim(),
        officialUrl,
        createdAt: Date.now(),
        status: "redirected",
      };

      this.redirectOrders.unshift(order);
      if (this.redirectOrders.length > 80) {
        this.redirectOrders = this.redirectOrders.slice(0, 80);
      }
      return order;
    },
    markOrderVisited(localId: string) {
      const target = this.redirectOrders.find((item) => item.localId === localId);
      if (!target) return;
      target.status = "visited";
    },
    archiveOrder(localId: string) {
      const target = this.redirectOrders.find((item) => item.localId === localId);
      if (!target) return;
      target.status = "archived";
    },
    restoreOrder(localId: string) {
      const target = this.redirectOrders.find((item) => item.localId === localId);
      if (!target) return;
      target.status = "visited";
    },
    clearArchivedOrders() {
      this.redirectOrders = this.redirectOrders.filter((item) => item.status !== "archived");
    },
  },
  persist: true,
});
