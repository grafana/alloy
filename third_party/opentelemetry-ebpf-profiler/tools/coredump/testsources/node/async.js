const meetCustomer = (id) => {
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      console.log(`Waiter approached customer at table #${id}...`);
      resolve({ customerId: id });
    }, id);
  });
}
const getOrder = (id) => {
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      console.log(`Order Received for customer at table #${id}...`);
      resolve({ customerId: id, customerOrder: "Pizza" });
    }, 1);
  });
}
const notifyWaiter = (id) => {
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      console.log(`Order for customer at table #${id} processed....`);
      resolve({ customerId: id, customerOrder: "Pizza" });
      // reject(new Error("Error occurred with waiter"));
    }, 1);
  });
}
const serveCustomer = (id) => {
  return new Promise((resolve, reject) => {
    for (let foo=0; foo <1000; foo++) {
	console.trace("I am here");
    }
    setTimeout(() => {
      console.log(`Customer with order number #${id} served...`);
      resolve({ customerId: id, customerOrder: "Pizza" });
    }, id);
  });
}

// Async- await approach 
const runRestaurant = async (customerId) => { 
  const customer = await meetCustomer(customerId) 
  const order = await getOrder(customer.customerId) 
  await notifyWaiter(order.customerId) 
  await serveCustomer(order.customerId) 
  console.log(`Order of customer fulfilled...`) 
} 

for (let i = 0; i < 1000; i++) {
 	console.log(`Order Received for customer at table #${i}...`);
	runRestaurant(i); 
}
