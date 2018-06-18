package network

import (
	"github.com/vishvananda/netlink"
)

//Setup is responsible for adding veth and connecting it to bridge
func Setup(pid int) error {

	// get bridge reference
	la := netlink.NewLinkAttrs()
	la.Name = "tst"
	mybridge := &netlink.Bridge{LinkAttrs: la}

	//create veth config
	veth := &netlink.Veth{
		PeerName:  "cnt-p2",
		LinkAttrs: netlink.LinkAttrs{Name: "cnt-p1"},
	}

	//add veth pair and get reference to new iterfaces
	err := netlink.LinkAdd(veth)
	if err != nil {
		return err
	}
	p1, err := netlink.LinkByName("cnt-p1")
	if err != nil {
		return err
	}
	p2, err := netlink.LinkByName("cnt-p2")
	if err != nil {
		return err
	}

	// add one of interfaces to bridge
	netlink.LinkSetMaster(p1, mybridge)

	//set first one up
	err = netlink.LinkSetUp(p1)
	if err != nil {
		return err
	}

	//move 2nd interface to our newly created namespace
	err = netlink.LinkSetNsPid(p2, pid)
	if err != nil {
		return err
	}

	return nil
}

//FinalConfig used to set interface after passing it to new ns
func FinalConfig() error {
	// get link reference
	p2, err := netlink.LinkByName("cnt-p2")
	if err != nil {
		return err
	}
	//set 2nd up (moving to ns clears interface settings)
	err = netlink.LinkSetUp(p2)
	if err != nil {
		return err
	}
	addr, err := netlink.ParseAddr("192.168.99.2/24")
	if err != nil {
		return err
	}
	err = netlink.AddrAdd(p2, addr)
	if err != nil {
		return err
	}
	return nil
}
